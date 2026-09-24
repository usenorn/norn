package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

const releaseNotes = `---
name: release-notes
description: Writes release notes from merged pull requests.
---

Read the merged pull requests since the last tag and group them by area.
`

func TestEverySourceSomeonePastesPointsAtTheSameSkill(t *testing.T) {
	cases := []struct {
		source string
		want   entity.AgentSkillLocation
	}{
		{"anthropics/skills", entity.AgentSkillLocation{Repository: "anthropics/skills"}},
		{"anthropics/skills/frontend-design", entity.AgentSkillLocation{Repository: "anthropics/skills", Skill: "frontend-design"}},
		{"https://skills.sh/anthropics/skills/frontend-design", entity.AgentSkillLocation{Repository: "anthropics/skills", Skill: "frontend-design"}},
		{"skills.sh/anthropics/skills", entity.AgentSkillLocation{Repository: "anthropics/skills"}},
		{"npx skills add anthropics/skills --skill frontend-design", entity.AgentSkillLocation{Repository: "anthropics/skills", Skill: "frontend-design"}},
		{"https://github.com/anthropics/skills.git", entity.AgentSkillLocation{Repository: "anthropics/skills"}},
		{
			"https://github.com/anthropics/skills/tree/main/skills/frontend-design",
			entity.AgentSkillLocation{Repository: "anthropics/skills", Ref: "main", Path: "skills/frontend-design"},
		},
		{
			"github.com/anthropics/skills/blob/v2/skills/pdf/SKILL.md",
			entity.AgentSkillLocation{Repository: "anthropics/skills", Ref: "v2", Path: "skills/pdf"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got, err := entity.ParseAgentSkillLocation(tc.source)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if got != tc.want {
				t.Errorf("location = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestASourceOutsideGitHubOrThatClimbsOutOfTheRepositoryIsRefused(t *testing.T) {
	for _, source := range []string{
		"",
		"just-a-name",
		"http://github.com/anthropics/skills",
		"https://gitlab.com/anthropics/skills",
		"https://github.com/anthropics/skills/tree/main/../../etc",
		"https://user:pass@github.com/anthropics/skills",
		"anthropics/../skills",
		"https://github.com/anthropics/skills/blob/main/README.md",
	} {
		var validation entity.ValidationError
		if _, err := entity.ParseAgentSkillLocation(source); !errors.As(err, &validation) {
			t.Errorf("%q: err = %v, want a validation error on source", source, err)
		}
	}
}

func TestAManifestNeedsANameADescriptionAndInstructions(t *testing.T) {
	manifest, err := entity.ParseAgentSkillManifest([]byte("\uFEFF" + strings.ReplaceAll(releaseNotes, "\n", "\r\n")))
	if err != nil {
		t.Fatalf("a manifest saved on Windows with a byte order mark was refused: %v", err)
	}

	if manifest.Name != "release-notes" || manifest.Description != "Writes release notes from merged pull requests." {
		t.Errorf("manifest = %+v", manifest)
	}

	cases := map[string]string{
		"no frontmatter":     "Just instructions.",
		"unclosed":           "---\nname: a\ndescription: b\n",
		"no description":     "---\nname: release-notes\n---\nBody.",
		"a name with spaces": "---\nname: Release Notes\ndescription: b\n---\nBody.",
		"no instructions":    "---\nname: release-notes\ndescription: b\n---\n\n",
	}

	for name, content := range cases {
		var validation entity.ValidationError
		if _, err := entity.ParseAgentSkillManifest([]byte(content)); !errors.As(err, &validation) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
}

func TestABundleMayNotWriteOutsideItsOwnDirectory(t *testing.T) {
	for _, path := range []string{"../escape.sh", "/etc/passwd", "scripts/../../escape", "a\\b", "./SKILL.md"} {
		bundle := entity.AgentSkillBundle{Files: []entity.AgentSkillFile{
			{Path: entity.AgentSkillManifestFile, Content: []byte(releaseNotes)},
			{Path: path, Content: []byte("x")},
		}}

		if _, err := bundle.Validate(); err == nil {
			t.Errorf("a bundle holding %q was accepted; unpacking it on a runner would write outside the skill", path)
		}
	}
}

func TestABundleHashIgnoresTheOrderFilesArrivedIn(t *testing.T) {
	first := entity.AgentSkillBundle{Files: []entity.AgentSkillFile{
		{Path: entity.AgentSkillManifestFile, Content: []byte(releaseNotes)},
		{Path: "scripts/collect.sh", Content: []byte("git log")},
	}}
	second := entity.AgentSkillBundle{Files: []entity.AgentSkillFile{first.Files[1], first.Files[0]}}

	if first.Hash() != second.Hash() {
		t.Error("the same files hashed differently, so re-importing an unchanged skill would look like a change")
	}

	second.Files[0].Content = []byte("git log --merges")
	if first.Hash() == second.Hash() {
		t.Error("a changed script kept the same hash, so the runner would keep a stale copy")
	}
}

func TestPickingASkillFromADiscovery(t *testing.T) {
	discovery := entity.AgentSkillDiscovery{Candidates: []entity.AgentSkillCandidate{
		{Name: "frontend-design", Path: "skills/frontend-design"},
		{Name: "pdf", Path: "skills/pdf-tools"},
	}}

	if picked, ok := discovery.Pick(entity.AgentSkillLocation{Skill: "pdf-tools"}); !ok || picked.Name != "pdf" {
		t.Errorf("by directory name: picked %+v %v", picked, ok)
	}

	if picked, ok := discovery.Pick(entity.AgentSkillLocation{Skill: "frontend-design"}); !ok || picked.Path != "skills/frontend-design" {
		t.Errorf("by frontmatter name: picked %+v %v", picked, ok)
	}

	if _, ok := discovery.Pick(entity.AgentSkillLocation{}); ok {
		t.Error("a repository with two skills picked one without being asked")
	}
}
