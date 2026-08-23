package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/service"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	bulkoperationsvc "github.com/usenorn/norn/internal/service/bulkoperation"
	changesetsvc "github.com/usenorn/norn/internal/service/changeset"
	codebasesvc "github.com/usenorn/norn/internal/service/codebase"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	delegationsvc "github.com/usenorn/norn/internal/service/delegation"
	directorysvc "github.com/usenorn/norn/internal/service/directory"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	executionuploadsvc "github.com/usenorn/norn/internal/service/executionupload"
	importssvc "github.com/usenorn/norn/internal/service/imports"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	issuequestionsvc "github.com/usenorn/norn/internal/service/issuequestion"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	licensingsvc "github.com/usenorn/norn/internal/service/licensing"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
	searchsvc "github.com/usenorn/norn/internal/service/search"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	ssoconnectionsvc "github.com/usenorn/norn/internal/service/ssoconnection"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	triagesvc "github.com/usenorn/norn/internal/service/triage"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const testMaxUploadBytes = 64

type importHarness struct {
	imports   *importssvc.MockImports
	routes    http.Handler
	workspace uuid.UUID
	run       uuid.UUID
}

func newImportHarness(t *testing.T) *importHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &importHarness{
		imports:   importssvc.NewMockImports(ctrl),
		workspace: uuid.New(),
		run:       uuid.New(),
	}

	edge := dashboard.New(
		accountsvc.NewMockAccounts(ctrl),
		workspacesvc.NewMockWorkspaces(ctrl),
		teamsvc.NewMockTeams(ctrl),
		invitationsvc.NewMockInvitations(ctrl),
		issuesvc.NewMockIssues(ctrl),
		issuerelationsvc.NewMockIssueRelations(ctrl),
		issuecommentsvc.NewMockIssueComments(ctrl),
		delegationsvc.NewMockDelegations(ctrl),
		issuequestionsvc.NewMockIssueQuestions(ctrl),
		runnersvc.NewMockRunners(ctrl),
		codebasesvc.NewMockCodebases(ctrl),
		executionsvc.NewMockExecutions(ctrl),
		executionuploadsvc.NewMockExecutionUploads(ctrl),
		changesetsvc.NewMockChangeSets(ctrl),
		attachmentsvc.NewMockAttachments(ctrl),
		bulkoperationsvc.NewMockBulkOperations(ctrl),
		workflowstatesvc.NewMockWorkflowStates(ctrl),
		labelsvc.NewMockLabels(ctrl),
		apitokensvc.NewMockAPITokens(ctrl),
		webhooksvc.NewMockWebhooks(ctrl),
		webhooksvc.NewMockWebhookDeliveries(ctrl),
		agentsvc.NewMockAgents(ctrl),
		sessionsvc.NewMockSessions(ctrl),
		ssoconnectionsvc.NewMockSSOConnections(ctrl),
		cyclesvc.NewMockCycles(ctrl),
		projectsvc.NewMockProjects(ctrl),
		savedviewsvc.NewMockSavedViews(ctrl),
		triagesvc.NewMockTriages(ctrl),
		notificationsvc.NewMockNotifications(ctrl),
		searchsvc.NewMockSearches(ctrl),
		auditsvc.NewMockAuditLog(ctrl),
		directorysvc.NewMockDirectories(ctrl),
		licensingsvc.NewMockLicensing(ctrl),
		h.imports,
		scmsvc.NewMockSourceControl(ctrl),
		scmsvc.NewMockSourceControlApps(ctrl),
		config.SourceControl{},
		config.App{Version: "test"},
		config.Instance{},
		config.Password{},
		config.Session{},
		config.Imports{MaxUploadBytes: testMaxUploadBytes},
	)

	h.routes = api.Handler(api.NewStrictHandler(edge, nil))

	return h
}

func (h *importHarness) call(t *testing.T, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, body)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	return recorder
}

func (h *importHarness) send(t *testing.T, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	return h.call(t, method, path, bytes.NewReader(encoded))
}

func (h *importHarness) base() string {
	return "/workspaces/" + h.workspace.String() + "/imports"
}

func (h *importHarness) at(suffix string) string {
	return h.base() + "/" + h.run.String() + suffix
}

func (h *importHarness) sampleRun() entity.ImportRun {
	return entity.ImportRun{
		ID:                 h.run,
		WorkspaceID:        h.workspace,
		SourceKind:         "linear",
		SourceLabel:        "Northwind",
		RequestedByAccount: uuid.New(),
		Status:             entity.ImportStaged,
		SourceSecretSet:    true,
		UnknownReferences:  entity.ImportUnknownSkip,
		Settings:           []byte(`{"teamKey":"CORE"}`),
		Staged:             120,
		Processed:          40,
		CreatedAt:          time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC),
	}
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var decoded T

	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode %s: %v", recorder.Body.String(), err)
	}

	return decoded
}

func TestASourceKeyNeverComesBackOutOfAnImportEndpoint(t *testing.T) {
	h := newImportHarness(t)

	const key = "lin_api_01HZXKQ9V4T7NORTHWIND"

	configured := h.sampleRun()

	h.imports.EXPECT().
		Configure(gomock.Any(), h.workspace, h.run, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.ConfigureImportInput,
		) (entity.ImportRun, error) {
			if input.Secret != key {
				t.Errorf("the service was handed secret %q, want the key the request carried", input.Secret)
			}

			return configured, nil
		})

	h.imports.EXPECT().Get(gomock.Any(), h.workspace, h.run).Return(configured, nil)
	h.imports.EXPECT().List(gomock.Any(), h.workspace, gomock.Any()).Return([]entity.ImportRun{configured}, nil)
	h.imports.EXPECT().
		Report(gomock.Any(), h.workspace, h.run).
		Return(service.ImportReportView{Run: configured}, nil)

	responses := map[string]*httptest.ResponseRecorder{
		"configure": h.send(t, http.MethodPut, h.at("/source"), map[string]any{"apiKey": key}),
		"get":       h.call(t, http.MethodGet, h.base()+"/"+h.run.String(), nil),
		"list":      h.call(t, http.MethodGet, h.base(), nil),
		"report":    h.call(t, http.MethodGet, h.at("/report"), nil),
	}

	for named, recorder := range responses {
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s = %d %s, want 200", named, recorder.Code, recorder.Body.String())
		}

		if strings.Contains(recorder.Body.String(), key) {
			t.Errorf(
				"%s carried the source key back out. Any member who may manage issues can call it, "+
					"and a Linear key reads that whole foreign workspace: %s",
				named, recorder.Body.String(),
			)
		}
	}

	run := decodeBody[api.ImportRun](t, responses["configure"])

	if !run.SourceSecretSet {
		t.Error("the run does not say a key is held, so a wizard would ask for it again on every visit")
	}
}

func TestAStalePreviewArrivesAsSomethingTheWizardCanActOn(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Execute(gomock.Any(), h.workspace, h.run, gomock.Any()).
		Return(entity.ImportRun{}, entity.ErrImportPreviewStale)

	recorder := h.send(t, http.MethodPost, h.at("/execute"), map[string]any{"previewDigest": "abc"})

	if recorder.Code != http.StatusConflict {
		t.Fatalf("execute = %d, want 409", recorder.Code)
	}

	problem := decodeBody[api.ImportConflictProblem](t, recorder)

	if problem.Code != api.ImportPreviewStale {
		t.Errorf(
			"code = %q, want %q. The screen has to know to re-fetch the preview rather than "+
				"only show a sentence",
			problem.Code, api.ImportPreviewStale,
		)
	}

	if problem.Detail == nil || *problem.Detail != entity.ErrImportPreviewStale.Error() {
		t.Errorf("detail = %v, want the domain's own sentence", problem.Detail)
	}
}

func TestARefusalToTriageNamesTheTeamsThatWouldSwallowTheWork(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Execute(gomock.Any(), h.workspace, h.run, gomock.Any()).
		Return(entity.ImportRun{}, entity.ImportWouldTriageError{Teams: []string{"Core", "Platform"}})

	recorder := h.send(t, http.MethodPost, h.at("/execute"), map[string]any{"previewDigest": "abc"})

	if recorder.Code != http.StatusConflict {
		t.Fatalf("execute = %d, want 409", recorder.Code)
	}

	problem := decodeBody[api.ImportConflictProblem](t, recorder)

	if problem.Code != api.ImportWouldTriage {
		t.Fatalf("code = %q, want %q", problem.Code, api.ImportWouldTriage)
	}

	if problem.Teams == nil || len(*problem.Teams) != 2 {
		t.Fatalf(
			"teams = %v. An operator asked to acknowledge triage cannot decide without knowing "+
				"which teams would swallow the import",
			problem.Teams,
		)
	}

	if (*problem.Teams)[0] != "Core" || (*problem.Teams)[1] != "Platform" {
		t.Errorf("teams = %v, want the teams the refusal named", *problem.Teams)
	}
}

func TestAnInstanceWithNoEncryptionKeySaysWhichVariableIsMissing(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Configure(gomock.Any(), h.workspace, h.run, gomock.Any()).
		Return(entity.ImportRun{}, entity.ErrImportEncryptionKeyMissing)

	recorder := h.send(t, http.MethodPut, h.at("/source"), map[string]any{"apiKey": "lin_api_01"})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"configure = %d, want 503. An instance started without a key is misconfigured "+
				"rather than broken",
			recorder.Code,
		)
	}

	problem := decodeBody[api.ImportSigningUnavailableProblem](t, recorder)

	if problem.Code != api.ImportSigningUnavailableProblemCodeImportSigningUnavailable {
		t.Errorf("code = %q, want import_signing_unavailable", problem.Code)
	}

	if problem.Detail == nil || !strings.Contains(*problem.Detail, "NORN_SECURITY_ENCRYPTION_KEY") {
		t.Errorf(
			"detail = %v. Whoever reads this is the person who can fix it, and they need the "+
				"name of the setting",
			problem.Detail,
		)
	}
}

func TestAFileLargerThanThisInstanceAcceptsIsRefusedRatherThanStored(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Upload(gomock.Any(), h.workspace, h.run, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, upload service.ImportUpload,
		) (service.ImportFile, error) {
			_, err := io.ReadAll(upload.Body)

			return service.ImportFile{}, err
		})

	body, contentType := multipartFile(t, "file", "backlog.csv", strings.Repeat("a", testMaxUploadBytes*2))

	request := httptest.NewRequest(http.MethodPost, h.at("/file"), body)
	request.Header.Set("Content-Type", contentType)

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("upload = %d %s, want 413", recorder.Code, recorder.Body.String())
	}
}

func TestAFileWithinTheLimitIsStoredUnderTheRunsOwnPrefix(t *testing.T) {
	h := newImportHarness(t)

	stored := service.ImportFile{
		ObjectKey: entity.ImportBlobKey(h.workspace, h.run, "backlog.csv"),
		FileName:  "backlog.csv",
	}

	h.imports.EXPECT().
		Upload(gomock.Any(), h.workspace, h.run, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, upload service.ImportUpload,
		) (service.ImportFile, error) {
			if upload.FileName != "backlog.csv" {
				t.Errorf("file name = %q, want the name the part carried", upload.FileName)
			}

			content, err := io.ReadAll(upload.Body)
			if err != nil {
				t.Fatalf("read part: %v", err)
			}

			if string(content) != "id,title\n" {
				t.Errorf("body = %q, want the bytes that were posted", content)
			}

			return stored, nil
		})

	body, contentType := multipartFile(t, "file", "backlog.csv", "id,title\n")

	request := httptest.NewRequest(http.MethodPost, h.at("/file"), body)
	request.Header.Set("Content-Type", contentType)

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("upload = %d %s, want 200", recorder.Code, recorder.Body.String())
	}

	file := decodeBody[api.ImportFile](t, recorder)

	if file.ObjectKey != stored.ObjectKey {
		t.Errorf(
			"objectKey = %q, want %q. Configuring the run is the next step and it addresses the "+
				"upload by this key",
			file.ObjectKey, stored.ObjectKey,
		)
	}
}

func TestTheRunListPagesByCursor(t *testing.T) {
	h := newImportHarness(t)

	first := h.sampleRun()
	second := h.sampleRun()
	second.ID = uuid.New()
	second.CreatedAt = first.CreatedAt.Add(-time.Hour)

	h.imports.EXPECT().
		List(gomock.Any(), h.workspace, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.ImportRunPage) ([]entity.ImportRun, error) {
			if page.Cursor != nil {
				t.Errorf("the first page asked for a cursor it was never given: %v", page.Cursor)
			}

			return []entity.ImportRun{first, second}, nil
		})

	opening := h.call(t, http.MethodGet, h.base()+"?limit=2", nil)

	if opening.Code != http.StatusOK {
		t.Fatalf("list = %d %s, want 200", opening.Code, opening.Body.String())
	}

	page := decodeBody[api.ImportRunPage](t, opening)

	if page.NextCursor == nil {
		t.Fatal("a full page carried no cursor, so the list stops at whatever the first page held")
	}

	h.imports.EXPECT().
		List(gomock.Any(), h.workspace, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, held entity.ImportRunPage) ([]entity.ImportRun, error) {
			if held.Cursor == nil {
				t.Fatal("the second page was asked for without a cursor")
			}

			if held.Cursor.RunID != second.ID || !held.Cursor.CreatedAt.Equal(second.CreatedAt) {
				t.Errorf(
					"cursor = %v, want the last run of the first page; anything else repeats or "+
						"skips rows",
					held.Cursor,
				)
			}

			return nil, nil
		})

	following := h.call(t, http.MethodGet, h.base()+"?limit=2&cursor="+*page.NextCursor, nil)

	if following.Code != http.StatusOK {
		t.Fatalf("second page = %d %s, want 200", following.Code, following.Body.String())
	}

	if rest := decodeBody[api.ImportRunPage](t, following); rest.NextCursor != nil {
		t.Errorf("a short page carried a cursor %q, so a reader would keep asking forever", *rest.NextCursor)
	}
}

func TestAnUnreadableCursorIsRefusedBeforeTheServiceIsCalled(t *testing.T) {
	h := newImportHarness(t)

	recorder := h.call(t, http.MethodGet, h.base()+"?cursor=not-a-cursor", nil)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("list = %d %s, want 422", recorder.Code, recorder.Body.String())
	}
}

func TestASourceThatRefusesSaysWhatItRefusedWith(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Catalogue(gomock.Any(), h.workspace, h.run).
		Return(entity.ImportCatalogue{}, entity.ImportSourceRefusedError{
			Resource: entity.ImportTeam,
			Reason:   "this token cannot read teams",
		})

	recorder := h.call(t, http.MethodGet, h.at("/catalogue"), nil)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("catalogue = %d %s, want 502", recorder.Code, recorder.Body.String())
	}

	problem := decodeBody[api.ImportSourceRefusedProblem](t, recorder)

	if problem.Reason == nil || *problem.Reason != "this token cannot read teams" {
		t.Errorf(
			"reason = %v. The refusal is the source's own and nothing on this side can restate it",
			problem.Reason,
		)
	}
}

func TestAThrottledSourceSaysHowLongToWait(t *testing.T) {
	h := newImportHarness(t)

	h.imports.EXPECT().
		Catalogue(gomock.Any(), h.workspace, h.run).
		Return(entity.ImportCatalogue{}, entity.ImportRateLimitedError{
			Resource:   entity.ImportTeam,
			RetryAfter: 90 * time.Second,
		})

	recorder := h.call(t, http.MethodGet, h.at("/catalogue"), nil)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("catalogue = %d %s, want 429", recorder.Code, recorder.Body.String())
	}

	if retry := recorder.Header().Get("Retry-After"); retry != "90" {
		t.Errorf("Retry-After = %q, want the wait the source asked for", retry)
	}
}

func TestEveryImportOperationRefusesACallerWhoMayNotManageIssues(t *testing.T) {
	denied := entity.AccessDeniedError{
		Reason:      entity.DenyReasonRoleLacksAction,
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: uuid.New(),
	}

	operations := []struct {
		name    string
		method  string
		suffix  string
		payload any
		expect  func(*importHarness)
	}{
		{
			name: "listImportSources", method: http.MethodGet, suffix: "/sources",
			expect: func(h *importHarness) {
				h.imports.EXPECT().Sources(gomock.Any(), h.workspace).Return(nil, denied)
			},
		},
		{
			name: "listWorkspaceImports", method: http.MethodGet, suffix: "",
			expect: func(h *importHarness) {
				h.imports.EXPECT().List(gomock.Any(), h.workspace, gomock.Any()).Return(nil, denied)
			},
		},
		{
			name: "createWorkspaceImport", method: http.MethodPost, suffix: "",
			payload: map[string]any{"sourceKind": "linear"},
			expect: func(h *importHarness) {
				h.imports.EXPECT().Connect(gomock.Any(), gomock.Any()).Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "getWorkspaceImport", method: http.MethodGet, suffix: "/{run}",
			expect: func(h *importHarness) {
				h.imports.EXPECT().Get(gomock.Any(), h.workspace, h.run).Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "configureWorkspaceImport", method: http.MethodPut, suffix: "/{run}/source",
			payload: map[string]any{"apiKey": "lin_api_01"},
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Configure(gomock.Any(), h.workspace, h.run, gomock.Any()).
					Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "getWorkspaceImportCatalogue", method: http.MethodGet, suffix: "/{run}/catalogue",
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Catalogue(gomock.Any(), h.workspace, h.run).
					Return(entity.ImportCatalogue{}, denied)
			},
		},
		{
			name: "stageWorkspaceImport", method: http.MethodPost, suffix: "/{run}/stage",
			expect: func(h *importHarness) {
				h.imports.EXPECT().Stage(gomock.Any(), h.workspace, h.run).Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "getWorkspaceImportMappings", method: http.MethodGet, suffix: "/{run}/mappings",
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Mappings(gomock.Any(), h.workspace, h.run).
					Return(entity.MappingPlan{}, denied)
			},
		},
		{
			name: "decideWorkspaceImportMappings", method: http.MethodPut, suffix: "/{run}/mappings",
			payload: map[string]any{"decisions": []any{}},
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Map(gomock.Any(), h.workspace, h.run, gomock.Any()).
					Return(entity.MappingPlan{}, denied)
			},
		},
		{
			name: "previewWorkspaceImport", method: http.MethodGet, suffix: "/{run}/preview",
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Preview(gomock.Any(), h.workspace, h.run).
					Return(entity.ImportPreview{}, denied)
			},
		},
		{
			name: "executeWorkspaceImport", method: http.MethodPost, suffix: "/{run}/execute",
			payload: map[string]any{"previewDigest": "abc"},
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Execute(gomock.Any(), h.workspace, h.run, gomock.Any()).
					Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "revertWorkspaceImport", method: http.MethodPost, suffix: "/{run}/revert",
			expect: func(h *importHarness) {
				h.imports.EXPECT().Revert(gomock.Any(), h.workspace, h.run).Return(entity.ImportRun{}, denied)
			},
		},
		{
			name: "getWorkspaceImportReport", method: http.MethodGet, suffix: "/{run}/report",
			expect: func(h *importHarness) {
				h.imports.EXPECT().
					Report(gomock.Any(), h.workspace, h.run).
					Return(service.ImportReportView{}, denied)
			},
		},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			h := newImportHarness(t)
			operation.expect(h)

			path := h.base() + strings.ReplaceAll(operation.suffix, "{run}", h.run.String())

			var recorder *httptest.ResponseRecorder

			if operation.payload == nil {
				recorder = h.call(t, operation.method, path, nil)
			} else {
				recorder = h.send(t, operation.method, path, operation.payload)
			}

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("%s = %d %s, want 403", operation.name, recorder.Code, recorder.Body.String())
			}
		})
	}

	t.Run("uploadWorkspaceImportFile", func(t *testing.T) {
		h := newImportHarness(t)

		h.imports.EXPECT().
			Upload(gomock.Any(), h.workspace, h.run, gomock.Any()).
			Return(service.ImportFile{}, denied)

		body, contentType := multipartFile(t, "file", "backlog.csv", "id\n")

		request := httptest.NewRequest(http.MethodPost, h.at("/file"), body)
		request.Header.Set("Content-Type", contentType)

		recorder := httptest.NewRecorder()
		h.routes.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("upload = %d %s, want 403", recorder.Code, recorder.Body.String())
		}
	})
}

func multipartFile(t *testing.T, field, name, content string) (io.Reader, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := fmt.Fprint(part, content); err != nil {
		t.Fatalf("write form file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}
