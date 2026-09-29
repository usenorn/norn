package channelv1

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

const (
	SkillBundleMaxBytes = 5 << 20
	SkillBundleMaxFiles = 200
	SkillManifestFile   = "SKILL.md"
)

type SkillFile struct {
	Path    string
	Content []byte
}

func SkillHash(files []SkillFile) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b SkillFile) int { return strings.Compare(a.Path, b.Path) })

	digest := sha256.New()
	for _, file := range sorted {
		content := sha256.Sum256(file.Content)
		digest.Write([]byte(file.Path))
		digest.Write([]byte{0})
		digest.Write(content[:])
	}

	return hex.EncodeToString(digest.Sum(nil))
}
