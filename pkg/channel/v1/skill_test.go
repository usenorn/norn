package channelv1_test

import (
	"testing"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestASkillHashIsTheSameWhateverOrderTheFilesArriveIn(t *testing.T) {
	manifest := channelv1.SkillFile{Path: "SKILL.md", Content: []byte("---\nname: triage\n---\n")}
	script := channelv1.SkillFile{Path: "scripts/run.sh", Content: []byte("echo triage\n")}

	stored := channelv1.SkillHash([]channelv1.SkillFile{manifest, script})
	unpacked := channelv1.SkillHash([]channelv1.SkillFile{script, manifest})

	if stored != unpacked {
		t.Fatalf(
			"hash %s after unpacking, %s when stored; a runner reads a tar in whatever order it "+
				"was written and would refuse a skill it was handed intact",
			unpacked, stored,
		)
	}

	script.Content = []byte("rm -rf /\n")

	if tampered := channelv1.SkillHash([]channelv1.SkillFile{manifest, script}); tampered == stored {
		t.Fatal("a changed file kept the hash, so a runner cannot tell it was not the skill norn stored")
	}
}
