package entity_test

import (
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestASectionEditTouchesOnlyItsOwnSection(t *testing.T) {
	const original = "# Guide\n\nIntro.\n\n## Setup\n\nOld steps.\n\n### Tools\n\nA hammer.\n\n## Usage\n\nRun it.\n"

	for name, testCase := range map[string]struct {
		heading     string
		replacement string
		want        string
	}{
		"a section takes its subsections with it": {
			heading:     "Setup",
			replacement: "## Setup\n\nNew steps.\n",
			want:        "# Guide\n\nIntro.\n\n## Setup\n\nNew steps.\n\n## Usage\n\nRun it.\n",
		},
		"the last section runs to the end": {
			heading:     "Usage",
			replacement: "## Usage\n\nRun it twice.\n",
			want:        "# Guide\n\nIntro.\n\n## Setup\n\nOld steps.\n\n### Tools\n\nA hammer.\n\n## Usage\n\nRun it twice.\n",
		},
		"a subsection stops at its parent's sibling": {
			heading:     "Tools",
			replacement: "### Tools\n\nA saw.\n",
			want:        "# Guide\n\nIntro.\n\n## Setup\n\nOld steps.\n\n### Tools\n\nA saw.\n\n## Usage\n\nRun it.\n",
		},
		"the heading may be written with its hashes": {
			heading:     "## Usage",
			replacement: "## Running\n\nRun it.\n",
			want:        "# Guide\n\nIntro.\n\n## Setup\n\nOld steps.\n\n### Tools\n\nA hammer.\n\n## Running\n\nRun it.\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			edited, err := entity.DocumentFromMarkdown(original).
				ReplaceSection(testCase.heading, entity.DocumentFromMarkdown(testCase.replacement))
			if err != nil {
				t.Fatalf("replace section: %v", err)
			}

			if got := edited.Markdown(); got != entity.DocumentFromMarkdown(testCase.want).Markdown() {
				t.Fatalf("got\n%s\nwant\n%s", got, testCase.want)
			}
		})
	}
}

func TestASectionEditNeedsExactlyOneHeadingToLandOn(t *testing.T) {
	for name, testCase := range map[string]struct {
		markdown string
		heading  string
		want     error
	}{
		"no such heading":               {markdown: "# Guide\n\nIntro.\n", heading: "Setup", want: entity.ErrDocSectionNotFound},
		"a hash inside code is not one": {markdown: "```\n# Setup\n```\n", heading: "Setup", want: entity.ErrDocSectionNotFound},
		"two headings say the same":     {markdown: "## Notes\n\nA.\n\n## Notes\n\nB.\n", heading: "Notes", want: entity.ErrDocSectionAmbiguous},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := entity.DocumentFromMarkdown(testCase.markdown).
				ReplaceSection(testCase.heading, entity.DocumentFromMarkdown("## Replaced\n"))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("got %v, want %v", err, testCase.want)
			}
		})
	}
}
