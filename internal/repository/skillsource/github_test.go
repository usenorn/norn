package skillsource_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/skillsource"
)

const revision = "9f1c0de5b2a4c6e8f0a1b3c5d7e9f1a3b5c7d9e1"

var files = map[string]string{
	"skills/pdf/SKILL.md":             "---\nname: pdf\ndescription: Fills PDF forms.\n---\n\nUse scripts/fill.py.\n",
	"skills/pdf/scripts/fill.py":      "print('filled')\n",
	"skills/frontend-design/SKILL.md": "---\nname: frontend-design\ndescription: Designs screens.\n---\n\nPick a direction.\n",
	"skills/broken/SKILL.md":          "no frontmatter at all",
	"README.md":                       "# skills\n",
}

const tree = `{"truncated":false,"tree":[
	{"path":"README.md","mode":"100644","type":"blob","size":10},
	{"path":"skills","mode":"040000","type":"tree"},
	{"path":"skills/pdf/SKILL.md","mode":"100644","type":"blob","size":70},
	{"path":"skills/pdf/scripts/fill.py","mode":"100755","type":"blob","size":17},
	{"path":"skills/pdf/link","mode":"120000","type":"blob","size":5},
	{"path":"skills/frontend-design/SKILL.md","mode":"100644","type":"blob","size":80},
	{"path":"skills/broken/SKILL.md","mode":"100644","type":"blob","size":21}
]}`

func source(t *testing.T) repository.SkillSource {
	t.Helper()

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/anthropics/skills/commits/HEAD":
			if r.Header.Get("Accept") != "application/vnd.github.sha" {
				t.Errorf("the commit lookup asked for %q, not the bare sha", r.Header.Get("Accept"))
			}

			_, _ = w.Write([]byte(revision))
		case "/repos/anthropics/skills/git/trees/" + revision:
			_, _ = w.Write([]byte(tree))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(github.Close)

	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := strings.CutPrefix(r.URL.Path, "/anthropics/skills/"+revision+"/")
		content, known := files[file]

		if !ok || !known {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(raw.Close)

	client, err := toolingclient.New(config.AgentTooling{
		GitHubEndpoint:      github.URL,
		GitHubRawEndpoint:   raw.URL,
		RegistryEndpoint:    github.URL,
		RequestTimeout:      5 * time.Second,
		DialTimeout:         time.Second,
		MaxResponseSize:     8 << 20,
		AllowedDestinations: []string{"127.0.0.1/32"},
	})
	if err != nil {
		t.Fatalf("toolingclient.New: %v", err)
	}

	return skillsource.New(client)
}

func TestDiscoveryListsEverySkillThatParsesAtOneCommit(t *testing.T) {
	discovery, err := source(t).Discover(context.Background(), entity.AgentSkillLocation{Repository: "anthropics/skills"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if discovery.Revision != revision {
		t.Errorf("revision = %q, want the commit HEAD pointed at", discovery.Revision)
	}

	names := []string{}
	for _, candidate := range discovery.Candidates {
		names = append(names, candidate.Name+"@"+candidate.Path)
	}

	want := "frontend-design@skills/frontend-design,pdf@skills/pdf"
	if strings.Join(names, ",") != want {
		t.Errorf("candidates = %v, want %s; a skill whose SKILL.md does not parse cannot be imported", names, want)
	}
}

func TestFetchingASkillKeepsItsFilesRelativeAndItsScriptsRunnable(t *testing.T) {
	bundle, err := source(t).Fetch(context.Background(), "anthropics/skills", revision, "skills/pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got := map[string]bool{}
	for _, file := range bundle.Files {
		got[file.Path] = file.Executable
	}

	if len(got) != 2 || got[entity.AgentSkillManifestFile] || !got["scripts/fill.py"] {
		t.Errorf("files = %v, want SKILL.md and an executable scripts/fill.py, and no symlink", got)
	}

	if _, err := bundle.Validate(); err != nil {
		t.Errorf("the fetched bundle does not validate: %v", err)
	}
}

func TestAMissingRepositoryIsNotFound(t *testing.T) {
	_, err := source(t).Discover(context.Background(), entity.AgentSkillLocation{Repository: "anthropics/nothing"})
	if !errors.Is(err, entity.ErrAgentSkillSourceNotFound) {
		t.Fatalf("err = %v, want ErrAgentSkillSourceNotFound", err)
	}
}
