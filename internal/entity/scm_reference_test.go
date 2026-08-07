package entity_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAReferenceIsReadOutOfWhateverTextTheForgeCarriesIt(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []entity.IssueReference
	}{
		{
			name: "a branch name puts the reference between two separators",
			text: "feature/eng-12-drop-the-cache",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}},
		},
		{
			name: "a branch name may be nothing but the reference",
			text: "ENG-12",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}},
		},
		{
			name: "a commit subject leads with the reference",
			text: "ENG-12: drop the cache",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}},
		},
		{
			name: "a commit body names it in a sentence",
			text: "This finally fixes ENG-12, which nobody could reproduce.",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}},
		},
		{
			name: "a body naming several yields each once, in the order they appear",
			text: "Closes ENG-12 and PLT-3. Follows on from ENG-12.",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}, {Key: "PLT", Number: 3}},
		},
		{
			name: "the key is normalised the way a typed reference is",
			text: "merge eng-12 into main",
			want: []entity.IssueReference{{Key: "ENG", Number: 12}},
		},
		{
			name: "a key longer than a team key can be is not a reference",
			text: "release/versions-12",
			want: []entity.IssueReference{},
		},
		{
			name: "a key shorter than a team key can be is not a reference",
			text: "a-12",
			want: []entity.IssueReference{},
		},
		{
			name: "a key carrying a digit is not a reference",
			text: "v2-15 shipped",
			want: []entity.IssueReference{},
		},
		{
			name: "a word too long to be a key is not a reference even where it ends in one",
			text: "planning-12",
			want: []entity.IssueReference{},
		},
		{
			name: "an ordinary word shaped like a key is a candidate, not a link",
			text: "utf-8",
			want: []entity.IssueReference{{Key: "UTF", Number: 8}},
		},
		{
			name: "a zero number is not an issue anybody has",
			text: "ENG-0",
			want: []entity.IssueReference{},
		},
		{
			name: "text with nothing reference-shaped yields nothing",
			text: "chore: bump dependencies and tidy the changelog",
			want: []entity.IssueReference{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := entity.ScanIssueReferences(testCase.text)

			if len(got) != len(testCase.want) {
				t.Fatalf("ScanIssueReferences(%q) = %v, want %v", testCase.text, got, testCase.want)
			}

			for i, reference := range testCase.want {
				if got[i] != reference {
					t.Errorf(
						"ScanIssueReferences(%q)[%d] = %v, want %v",
						testCase.text, i, got[i], reference,
					)
				}
			}
		})
	}
}

func TestTheScannerReadsWhatAWholeReferenceParserCannot(t *testing.T) {
	const branch = "eric/ENG-12-rewrite-the-importer"

	if _, err := entity.ParseIssueReference(branch); err == nil {
		t.Fatal("ParseIssueReference accepted a branch name; the scanner would then be pointless")
	}

	found := entity.ScanIssueReferences(branch)

	if len(found) != 1 || found[0] != (entity.IssueReference{Key: "ENG", Number: 12}) {
		t.Fatalf("ScanIssueReferences(%q) = %v, want one ENG-12", branch, found)
	}
}

func TestOneMessageCannotLinkAnIssueToEverythingInTheWorkspace(t *testing.T) {
	var body strings.Builder

	for i := 1; i <= entity.CodeLinkMaxReferences*3; i++ {
		fmt.Fprintf(&body, "ENG-%d ", i)
	}

	found := entity.ScanIssueReferences(body.String())

	if len(found) != entity.CodeLinkMaxReferences {
		t.Fatalf(
			"ScanIssueReferences found %d references, want the cap of %d — a generated commit "+
				"body must not be able to attach one change to an unbounded number of issues",
			len(found), entity.CodeLinkMaxReferences,
		)
	}
}

func TestTheScannerRefusesToReadAnUnboundedBody(t *testing.T) {
	buried := strings.Repeat("x", entity.CodeLinkScanMaxLen) + " ENG-12"

	if found := entity.ScanIssueReferences(buried); len(found) != 0 {
		t.Fatalf(
			"ScanIssueReferences read %v past the %d-byte cap; the cap is what bounds the work "+
				"a hostile commit message can ask for",
			found, entity.CodeLinkScanMaxLen,
		)
	}
}
