package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type ImportPreviewLine struct {
	Resource   ImportResource
	Subject    string
	ExternalID string
	Outcome    ImportOutcome
	Detail     string
}

type ImportPreview struct {
	Created      []ImportPreviewLine
	Changed      []ImportPreviewLine
	Skipped      []ImportPreviewLine
	Unattributed []ImportPreviewLine
	Warnings     []ImportPreviewLine
	TriageTeams  []string
}

func (p ImportPreview) Empty() bool {
	return len(p.Created) == 0 && len(p.Changed) == 0
}

func (p ImportPreview) WouldTriage() bool {
	return len(p.TriageTeams) > 0
}

// Digest fingerprints what this preview says would happen. Execute recomputes it and
// refuses a run whose digest has moved, which is what makes preview safe to keep pure:
// a target deleted between previewing and executing changes the fingerprint.
func (p ImportPreview) Digest() string {
	sum := sha256.New()

	for _, group := range []struct {
		name  string
		lines []ImportPreviewLine
	}{
		{"created", p.Created},
		{"changed", p.Changed},
		{"skipped", p.Skipped},
		{"unattributed", p.Unattributed},
		{"warnings", p.Warnings},
	} {
		encoded := make([]string, 0, len(group.lines))

		for _, line := range group.lines {
			encoded = append(encoded, fmt.Sprintf("%s|%s|%s|%s|%s",
				line.Resource, line.ExternalID, line.Outcome, line.Subject, line.Detail))
		}

		sort.Strings(encoded)

		sum.Write([]byte(group.name + "\n" + strings.Join(encoded, "\n") + "\n"))
	}

	teams := append([]string(nil), p.TriageTeams...)
	sort.Strings(teams)
	sum.Write([]byte("triage\n" + strings.Join(teams, "\n") + "\n"))

	return hex.EncodeToString(sum.Sum(nil))
}
