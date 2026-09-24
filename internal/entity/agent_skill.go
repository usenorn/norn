package entity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	AgentCapabilityNameMaxLen     = 64
	AgentSkillDescriptionMaxLen   = 1024
	AgentSkillSourceMaxLen        = 512
	AgentSkillBundleMaxBytes      = 5 << 20
	AgentSkillBundleMaxFiles      = 200
	AgentSkillFilePathMaxLen      = 256
	AgentSkillsPerOwner           = 50
	AgentSkillCandidatesMax       = 100
	AgentSkillManifestFile        = "SKILL.md"
	AgentSkillBundleContentType   = "application/gzip"
	agentSkillFrontmatterBoundary = "---"
	githubHost                    = "github.com"
	skillsDirectoryHost           = "skills.sh"
	skillsInstallCommand          = "npx skills add"
	skillsInstallSkillFlag        = "--skill"
	byteOrderMark                 = "\uFEFF"
)

var (
	ErrAgentSkillNotFound                  = errors.New("skill not found")
	ErrAgentSkillNameTaken                 = errors.New("this agent already has a skill with that name")
	ErrAgentSkillLimitReached              = errors.New("no more skills can be added here")
	ErrAgentSkillSourceNotFound            = errors.New("no skill was found at that source")
	ErrAgentSkillSourceUnreachable         = errors.New("the skill source could not be reached")
	ErrAgentSkillNotImported               = errors.New("this skill was written here, so there is no source to update it from")
	ErrAgentSkillImported                  = errors.New("this skill is imported, so it is updated from its source")
	ErrAgentCapabilityAttached             = errors.New("this agent already uses that library item")
	ErrAgentCapabilityNotAttached          = errors.New("this agent does not use that library item")
	ErrAgentCapabilityNotLibrary           = errors.New("only a library item can be attached to an agent")
	ErrAgentCapabilityEncryptionKeyMissing = errors.New("this instance has no encryption key, so secrets cannot be stored")

	agentCapabilityName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	githubSegment       = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func ValidateAgentCapabilityName(field, name string) FieldError {
	switch {
	case name == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(name) > AgentCapabilityNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !agentCapabilityName.MatchString(name):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

type AgentSkillSource string

const (
	AgentSkillManual AgentSkillSource = "manual"
	AgentSkillGitHub AgentSkillSource = "github"
)

func (s AgentSkillSource) Valid() bool {
	return s == AgentSkillManual || s == AgentSkillGitHub
}

type AgentSkillOrigin struct {
	Repository string
	Path       string
	Ref        string
	Revision   string
}

type AgentSkill struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	AgentID      *uuid.UUID
	Name         string
	Description  string
	Source       AgentSkillSource
	Origin       AgentSkillOrigin
	Instructions string
	ContentHash  string
	ObjectKey    string
	SizeBytes    int64
	FileCount    int
	CreatedBy    uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s AgentSkill) InLibrary() bool {
	return s.AgentID == nil
}

func AgentSkillObjectKey(workspaceID, skillID uuid.UUID, contentHash string) string {
	return "agent-skills/" + workspaceID.String() + "/" + skillID.String() + "/" + contentHash + ".tar.gz"
}

type AgentSkillFile struct {
	Path       string
	Executable bool
	Content    []byte
}

type AgentSkillBundle struct {
	Files []AgentSkillFile
}

type AgentSkillManifest struct {
	Name         string
	Description  string
	Instructions string
}

func (b AgentSkillBundle) Size() int64 {
	var total int64
	for _, file := range b.Files {
		total += int64(len(file.Content))
	}

	return total
}

func (b AgentSkillBundle) Hash() string {
	files := slices.Clone(b.Files)
	slices.SortFunc(files, func(a, c AgentSkillFile) int { return strings.Compare(a.Path, c.Path) })

	digest := sha256.New()
	for _, file := range files {
		content := sha256.Sum256(file.Content)
		digest.Write([]byte(file.Path))
		digest.Write([]byte{0})
		digest.Write(content[:])
	}

	return hex.EncodeToString(digest.Sum(nil))
}

func (b AgentSkillBundle) Validate() (AgentSkillManifest, error) {
	if len(b.Files) == 0 {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "bundle", Code: ValidationCodeRequired})
	}

	if len(b.Files) > AgentSkillBundleMaxFiles || b.Size() > AgentSkillBundleMaxBytes {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "bundle", Code: ValidationCodeTooLong})
	}

	seen := make(map[string]bool, len(b.Files))

	var manifest []byte

	for _, file := range b.Files {
		if !validBundlePath(file.Path) || seen[file.Path] {
			return AgentSkillManifest{}, NewValidationError(FieldError{Field: "bundle", Code: ValidationCodeMalformed})
		}

		seen[file.Path] = true

		if file.Path == AgentSkillManifestFile {
			manifest = file.Content
		}
	}

	if manifest == nil {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "instructions", Code: ValidationCodeRequired})
	}

	return ParseAgentSkillManifest(manifest)
}

func validBundlePath(name string) bool {
	return name != "" &&
		len(name) <= AgentSkillFilePathMaxLen &&
		utf8.ValidString(name) &&
		!strings.HasPrefix(name, "/") &&
		!strings.Contains(name, "\\") &&
		path.Clean(name) == name &&
		name != ".." &&
		!strings.HasPrefix(name, "../")
}

type agentSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ParseAgentSkillManifest(content []byte) (AgentSkillManifest, error) {
	if !utf8.Valid(content) {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "instructions", Code: ValidationCodeMalformed})
	}

	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimPrefix(text, byteOrderMark)

	if !strings.HasPrefix(text, agentSkillFrontmatterBoundary+"\n") {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "instructions", Code: ValidationCodeMalformed})
	}

	rest := text[len(agentSkillFrontmatterBoundary)+1:]

	end := strings.Index(rest, "\n"+agentSkillFrontmatterBoundary)
	if end < 0 {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "instructions", Code: ValidationCodeMalformed})
	}

	var front agentSkillFrontmatter
	if err := yaml.NewDecoder(bytes.NewReader([]byte(rest[:end]))).Decode(&front); err != nil {
		return AgentSkillManifest{}, NewValidationError(FieldError{Field: "instructions", Code: ValidationCodeMalformed})
	}

	body := rest[end+len(agentSkillFrontmatterBoundary)+1:]
	body = strings.TrimPrefix(body, "\n")

	manifest := AgentSkillManifest{
		Name:         strings.TrimSpace(front.Name),
		Description:  strings.TrimSpace(front.Description),
		Instructions: text,
	}

	fields := []FieldError{ValidateAgentCapabilityName("name", manifest.Name)}

	switch {
	case manifest.Description == "":
		fields = append(fields, FieldError{Field: "description", Code: ValidationCodeRequired})
	case utf8.RuneCountInString(manifest.Description) > AgentSkillDescriptionMaxLen:
		fields = append(fields, FieldError{Field: "description", Code: ValidationCodeTooLong})
	}

	if strings.TrimSpace(body) == "" {
		fields = append(fields, FieldError{Field: "instructions", Code: ValidationCodeRequired})
	}

	if err := NewValidationError(fields...); err != nil {
		return AgentSkillManifest{}, err
	}

	return manifest, nil
}

type AgentSkillLocation struct {
	Repository string
	Ref        string
	Path       string
	Skill      string
}

func sourceInvalid() error {
	return NewValidationError(FieldError{Field: "source", Code: ValidationCodeMalformed})
}

func ParseAgentSkillLocation(raw string) (AgentSkillLocation, error) {
	source := strings.TrimSpace(raw)

	switch {
	case source == "":
		return AgentSkillLocation{}, NewValidationError(FieldError{Field: "source", Code: ValidationCodeRequired})
	case len(source) > AgentSkillSourceMaxLen:
		return AgentSkillLocation{}, NewValidationError(FieldError{Field: "source", Code: ValidationCodeTooLong})
	}

	if command, ok := strings.CutPrefix(source, skillsInstallCommand); ok {
		return parseInstallCommand(strings.Fields(command))
	}

	if !strings.Contains(source, "://") {
		if strings.HasPrefix(source, githubHost+"/") || strings.HasPrefix(source, skillsDirectoryHost+"/") {
			source = "https://" + source
		} else {
			return parseShorthand(strings.Split(strings.Trim(source, "/"), "/"))
		}
	}

	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return AgentSkillLocation{}, sourceInvalid()
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	switch strings.ToLower(parsed.Hostname()) {
	case skillsDirectoryHost:
		return parseShorthand(segments)
	case githubHost, "www." + githubHost:
		return parseGitHubURL(segments)
	}

	return AgentSkillLocation{}, sourceInvalid()
}

func parseInstallCommand(arguments []string) (AgentSkillLocation, error) {
	if len(arguments) == 0 {
		return AgentSkillLocation{}, sourceInvalid()
	}

	location, err := ParseAgentSkillLocation(arguments[0])
	if err != nil {
		return AgentSkillLocation{}, err
	}

	for i := 1; i < len(arguments); i++ {
		if arguments[i] == skillsInstallSkillFlag && i+1 < len(arguments) {
			location.Skill = arguments[i+1]
			i++
		}
	}

	return location, nil
}

func parseShorthand(segments []string) (AgentSkillLocation, error) {
	if len(segments) < 2 || len(segments) > 3 {
		return AgentSkillLocation{}, sourceInvalid()
	}

	for _, segment := range segments {
		if !githubSegment.MatchString(segment) || segment == "." || segment == ".." {
			return AgentSkillLocation{}, sourceInvalid()
		}
	}

	location := AgentSkillLocation{Repository: segments[0] + "/" + strings.TrimSuffix(segments[1], ".git")}
	if len(segments) == 3 {
		location.Skill = segments[2]
	}

	return location, nil
}

func parseGitHubURL(segments []string) (AgentSkillLocation, error) {
	location, err := parseShorthand(segments[:min(2, len(segments))])
	if err != nil {
		return AgentSkillLocation{}, err
	}

	if len(segments) == 2 {
		return location, nil
	}

	if len(segments) < 4 || (segments[2] != "tree" && segments[2] != "blob") {
		return AgentSkillLocation{}, sourceInvalid()
	}

	location.Ref = segments[3]

	directory := strings.Join(segments[4:], "/")
	if segments[2] == "blob" {
		if path.Base(directory) != AgentSkillManifestFile {
			return AgentSkillLocation{}, sourceInvalid()
		}

		directory = path.Dir(directory)
	}

	if directory == "." {
		directory = ""
	}

	if directory != "" && !validBundlePath(directory) {
		return AgentSkillLocation{}, sourceInvalid()
	}

	location.Path = directory

	return location, nil
}

type AgentSkillCandidate struct {
	Name        string
	Description string
	Path        string
}

type AgentSkillDiscovery struct {
	Location   AgentSkillLocation
	Revision   string
	Candidates []AgentSkillCandidate
}

func (d AgentSkillDiscovery) Pick(location AgentSkillLocation) (AgentSkillCandidate, bool) {
	for _, candidate := range d.Candidates {
		if location.Path != "" && candidate.Path == location.Path {
			return candidate, true
		}

		if location.Skill != "" && (candidate.Name == location.Skill || path.Base(candidate.Path) == location.Skill) {
			return candidate, true
		}
	}

	if location.Path == "" && location.Skill == "" && len(d.Candidates) == 1 {
		return d.Candidates[0], true
	}

	return AgentSkillCandidate{}, false
}

type AgentCapabilityAttachment struct {
	WorkspaceID  uuid.UUID
	AgentID      uuid.UUID
	CapabilityID uuid.UUID
	AttachedBy   uuid.UUID
}
