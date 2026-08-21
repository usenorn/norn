package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	CodebaseNameMaxLen        = 200
	CodebaseRootPathMaxLen    = 1024
	CodebaseRepositoryMaxLen  = 200
	CodebaseRelPathMaxLen     = 1024
	CodebaseBranchMaxLen      = 255
	CodebaseRemoteHashMaxLen  = 128
	CodebaseRemoteHostMaxLen  = 255
	CodebasePathTailMaxLen    = 255
	CodebaseSharedFileMaxLen  = 255
	CodebaseToolNameMaxLen    = 64
	CodebaseToolVersionMaxLen = 64

	CodebaseMaxRepositories = 200
	CodebaseMaxSharedFiles  = 100
	CodebaseMaxTools        = 32
)

var (
	ErrCodebaseNotFound     = errors.New("codebase not found")
	ErrCodebaseRootTaken    = errors.New("this runner already has a codebase connected at that path")
	ErrCodebaseNotDrifted   = errors.New("this codebase has no drift to confirm")
	ErrCodebaseDisconnected = errors.New("this codebase has been disconnected")
	ErrCodebaseNotRunner    = errors.New("only a runner may connect a codebase")
)

type CodebaseState string

const (
	CodebaseStateActive       CodebaseState = "active"
	CodebaseStateDrift        CodebaseState = "drift"
	CodebaseStateDisconnected CodebaseState = "disconnected"
)

func CodebaseStates() []CodebaseState {
	return []CodebaseState{CodebaseStateActive, CodebaseStateDrift, CodebaseStateDisconnected}
}

func (s CodebaseState) Valid() bool {
	return slices.Contains(CodebaseStates(), s)
}

type CodebaseRuntime string

const (
	CodebaseRuntimeProcess CodebaseRuntime = "process"
	CodebaseRuntimeDocker  CodebaseRuntime = "docker"
	CodebaseRuntimeKVM     CodebaseRuntime = "kvm"
)

func CodebaseRuntimes() []CodebaseRuntime {
	return []CodebaseRuntime{CodebaseRuntimeProcess, CodebaseRuntimeDocker, CodebaseRuntimeKVM}
}

func (r CodebaseRuntime) Valid() bool {
	return slices.Contains(CodebaseRuntimes(), r)
}

type RemoteFingerprint struct {
	Hash     string
	Host     string
	PathTail string
}

type CodebaseRepository struct {
	Name          string
	RelPath       string
	DefaultBranch string
	Remote        RemoteFingerprint
}

type CodingTool struct {
	Name    string
	Version string
}

type Codebase struct {
	ID             uuid.UUID
	RunnerID       uuid.UUID
	WorkspaceID    uuid.UUID
	AgentID        uuid.UUID
	Name           string
	RootPath       string
	State          CodebaseState
	Repositories   []CodebaseRepository
	SharedFiles    []string
	Runtimes       []CodebaseRuntime
	Tools          []CodingTool
	ConnectedAt    time.Time
	LastSeenAt     *time.Time
	DisconnectedAt *time.Time
	UpdatedAt      time.Time
}

func (c Codebase) Disconnected() bool {
	return c.State == CodebaseStateDisconnected
}

func (c Codebase) Drifted() bool {
	return c.State == CodebaseStateDrift
}

func SameRepositorySet(before, after []CodebaseRepository) bool {
	if len(before) != len(after) {
		return false
	}

	held := make(map[string]CodebaseRepository, len(before))
	for _, repository := range before {
		held[repository.RelPath] = repository
	}

	for _, repository := range after {
		previous, ok := held[repository.RelPath]
		if !ok || previous != repository {
			return false
		}
	}

	return true
}

func codebaseText(field, value string, max int) FieldError {
	trimmed := strings.TrimSpace(value)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > max:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func codebaseOptionalText(field, value string, max int) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(value)) > max {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateCodebaseName(field, name string) FieldError {
	return codebaseText(field, name, CodebaseNameMaxLen)
}

func ValidateCodebaseRootPath(field, path string) FieldError {
	return codebaseText(field, path, CodebaseRootPathMaxLen)
}

func ValidateCodebaseRepositories(field string, repositories []CodebaseRepository) []FieldError {
	if len(repositories) > CodebaseMaxRepositories {
		return []FieldError{{Field: field, Code: ValidationCodeOutOfRange}}
	}

	fields := make([]FieldError, 0, len(repositories))
	seen := make(map[string]struct{}, len(repositories))

	for _, repository := range repositories {
		path := strings.TrimSpace(repository.RelPath)

		if _, duplicate := seen[path]; duplicate && path != "" {
			return []FieldError{{Field: field, Code: ValidationCodeUnsupportedValue}}
		}

		seen[path] = struct{}{}

		fields = append(fields,
			codebaseText(field+".name", repository.Name, CodebaseRepositoryMaxLen),
			codebaseText(field+".relPath", repository.RelPath, CodebaseRelPathMaxLen),
			codebaseOptionalText(field+".defaultBranch", repository.DefaultBranch, CodebaseBranchMaxLen),
			codebaseOptionalText(field+".remote.hash", repository.Remote.Hash, CodebaseRemoteHashMaxLen),
			codebaseOptionalText(field+".remote.host", repository.Remote.Host, CodebaseRemoteHostMaxLen),
			codebaseOptionalText(field+".remote.pathTail", repository.Remote.PathTail, CodebasePathTailMaxLen),
		)
	}

	return fields
}

func ValidateCodebaseSharedFiles(field string, files []string) []FieldError {
	if len(files) > CodebaseMaxSharedFiles {
		return []FieldError{{Field: field, Code: ValidationCodeOutOfRange}}
	}

	fields := make([]FieldError, 0, len(files))
	for _, file := range files {
		fields = append(fields, codebaseText(field, file, CodebaseSharedFileMaxLen))
	}

	return fields
}

func ValidateCodebaseRuntimes(field string, runtimes []CodebaseRuntime) []FieldError {
	fields := make([]FieldError, 0, len(runtimes))

	for _, runtime := range runtimes {
		if !runtime.Valid() {
			fields = append(fields, FieldError{Field: field, Code: ValidationCodeUnsupportedValue})
		}
	}

	return fields
}

func ValidateCodebaseTools(field string, tools []CodingTool) []FieldError {
	if len(tools) > CodebaseMaxTools {
		return []FieldError{{Field: field, Code: ValidationCodeOutOfRange}}
	}

	fields := make([]FieldError, 0, len(tools))
	for _, tool := range tools {
		fields = append(fields,
			codebaseText(field+".name", tool.Name, CodebaseToolNameMaxLen),
			codebaseOptionalText(field+".version", tool.Version, CodebaseToolVersionMaxLen),
		)
	}

	return fields
}
