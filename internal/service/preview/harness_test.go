package preview_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	executionrepo "github.com/usenorn/norn/internal/repository/execution"
	previewrepo "github.com/usenorn/norn/internal/repository/preview"
	grantrepo "github.com/usenorn/norn/internal/repository/previewgrant"
	sharerepo "github.com/usenorn/norn/internal/repository/previewshare"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	previewsvc "github.com/usenorn/norn/internal/service/preview"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	previewDomain = "preview.norn.test"
	appBaseURL    = "https://norn.test"
)

type harness struct {
	previews   *previewrepo.MockPreview
	shares     *sharerepo.MockPreviewShare
	grants     *grantrepo.MockPreviewGrant
	executions *executionrepo.MockExecution
	runs       *executionsvc.MockExecutions
	events     *eventsvc.MockEvents
	authorizer *authorizersvc.MockAuthorizer
	audit      *auditsvc.MockAudit
	service    service.Previews

	workspaceID uuid.UUID
	execution   entity.Execution
	runner      entity.Runner
	caller      uuid.UUID

	stored   map[string]entity.PreviewSession
	links    map[uuid.UUID]entity.PreviewShareLink
	issued   map[string]entity.PreviewGrant
	tickets  map[string]entity.PreviewGrant
	recorded []entity.ExecutionEvent
	audited  []entity.AuditEntry
	looks    map[string]bool
	attempts map[string]int
	shutOut  []uuid.UUID
}

func newHarness(t *testing.T) *harness {
	return newHarnessServing(t, previewDomain)
}

func newHarnessServing(t *testing.T, domain string) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		previews:    previewrepo.NewMockPreview(ctrl),
		shares:      sharerepo.NewMockPreviewShare(ctrl),
		grants:      grantrepo.NewMockPreviewGrant(ctrl),
		executions:  executionrepo.NewMockExecution(ctrl),
		runs:        executionsvc.NewMockExecutions(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		workspaceID: workspaceID,
		caller:      uuid.New(),
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
		stored:   map[string]entity.PreviewSession{},
		links:    map[uuid.UUID]entity.PreviewShareLink{},
		issued:   map[string]entity.PreviewGrant{},
		tickets:  map[string]entity.PreviewGrant{},
		looks:    map[string]bool{},
		attempts: map[string]int{},
	}

	h.execution = entity.Execution{
		ID:          "exec-01ABC",
		WorkspaceID: workspaceID,
		IssueID:     uuid.New(),
		TeamID:      teamID,
		AgentID:     agentID,
		RunnerID:    h.runner.ID,
		State:       entity.ExecutionRunning,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.expectStore()
	h.expectGrants()

	h.events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	h.audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) {
			h.audited = append(h.audited, entry)
		}).
		AnyTimes()

	h.executions.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) (entity.Execution, error) {
			if executionID != h.execution.ID {
				return entity.Execution{}, entity.ErrExecutionNotFound
			}

			return h.execution, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		AppendEvent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, event entity.ExecutionEvent,
		) (entity.ExecutionEvent, error) {
			for _, recorded := range h.recorded {
				if event.SourceID != "" && recorded.SourceID == event.SourceID {
					return entity.ExecutionEvent{}, entity.ErrExecutionEventRecorded
				}
			}

			event.ID = uuid.New()
			event.Sequence = int64(len(h.recorded)) + 1
			h.recorded = append(h.recorded, event)

			return event, nil
		}).
		AnyTimes()

	h.service = previewsvc.New(
		h.previews, h.shares, h.grants, h.executions, h.runs, h.events, h.authorizer, h.audit,
		transactor,
		config.App{BaseURL: appBaseURL},
		config.Previews{
			BaseDomain:      domain,
			Scheme:          "https",
			SessionTTL:      15 * time.Minute,
			TicketTTL:       time.Minute,
			ShareDefaultTTL: 24 * time.Hour,
			ShareMaxTTL:     7 * 24 * time.Hour,
			AuditWindow:     time.Hour,
		},
	)

	return h
}

func (h *harness) holds() {
	h.runs.EXPECT().
		Held(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, runner entity.Runner, executionID string,
		) (entity.Execution, error) {
			if runner.ID != h.runner.ID || executionID != h.execution.ID {
				return entity.Execution{}, entity.ErrExecutionNotFound
			}

			return h.execution, nil
		}).
		AnyTimes()
}

func (h *harness) visible(err error) {
	h.runs.EXPECT().
		Visible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ string,
		) (entity.Execution, error) {
			if err != nil {
				return entity.Execution{}, err
			}

			return h.execution, nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Manageable(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ string,
		) (entity.Execution, error) {
			if err != nil {
				return entity.Execution{}, err
			}

			return h.execution, nil
		}).
		AnyTimes()
}

func (h *harness) expectStore() {
	h.previews.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, preview entity.PreviewSession,
		) (entity.PreviewSession, error) {
			held, known := h.stored[preview.Name]
			if known {
				if preview.ReportedAt.Before(held.ReportedAt) {
					return held, nil
				}

				preview.ID = held.ID
				preview.CreatedAt = held.CreatedAt
			} else {
				preview.ID = uuid.New()
			}

			h.stored[preview.Name] = preview

			return preview, nil
		}).
		AnyTimes()

	h.previews.EXPECT().
		ByName(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, name string,
		) (entity.PreviewSession, error) {
			held, known := h.stored[name]
			if !known {
				return entity.PreviewSession{}, entity.ErrPreviewNotFound
			}

			return held, nil
		}).
		AnyTimes()

	h.previews.EXPECT().
		ByHost(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, host string) (entity.PreviewSession, error) {
			for _, held := range h.stored {
				if held.Host != "" && held.Host == host {
					return held, nil
				}
			}

			return entity.PreviewSession{}, entity.ErrPreviewNotFound
		}).
		AnyTimes()

	h.previews.EXPECT().
		ByExecution(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) ([]entity.PreviewSession, error) {
			held := make([]entity.PreviewSession, 0, len(h.stored))

			for _, preview := range h.stored {
				held = append(held, preview)
			}

			return held, nil
		}).
		AnyTimes()

	h.previews.EXPECT().
		Count(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (int, error) {
			return len(h.stored), nil
		}).
		AnyTimes()

	h.shares.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, link entity.PreviewShareLink,
		) (entity.PreviewShareLink, error) {
			link.ID = uuid.New()
			h.links[link.ID] = link

			return link, nil
		}).
		AnyTimes()

	h.shares.EXPECT().
		ByToken(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hash []byte) (entity.PreviewShareLink, error) {
			for _, link := range h.links {
				if string(link.TokenHash) == string(hash) {
					return link, nil
				}
			}

			return entity.PreviewShareLink{}, entity.ErrPreviewShareNotFound
		}).
		AnyTimes()

	h.shares.EXPECT().
		ByPreview(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, previewID uuid.UUID) ([]entity.PreviewShareLink, error) {
			held := make([]entity.PreviewShareLink, 0, len(h.links))

			for _, link := range h.links {
				if link.PreviewID == previewID {
					held = append(held, link)
				}
			}

			return held, nil
		}).
		AnyTimes()

	h.shares.EXPECT().
		ByExecution(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) ([]entity.PreviewShareLink, error) {
			held := make([]entity.PreviewShareLink, 0, len(h.links))

			for _, link := range h.links {
				held = append(held, link)
			}

			return held, nil
		}).
		AnyTimes()

	h.shares.EXPECT().
		Revoke(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, previewID, linkID uuid.UUID, at time.Time,
		) (entity.PreviewShareLink, error) {
			link, known := h.links[linkID]
			if !known || link.PreviewID != previewID {
				return entity.PreviewShareLink{}, entity.ErrPreviewShareNotFound
			}

			link.RevokedAt = at
			h.links[linkID] = link

			return link, nil
		}).
		AnyTimes()

	h.shares.EXPECT().
		Used(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, linkID uuid.UUID, at time.Time) error {
			link, known := h.links[linkID]
			if !known {
				return entity.ErrPreviewShareNotFound
			}

			link.Uses++
			link.LastUsedAt = at
			h.links[linkID] = link

			return nil
		}).
		AnyTimes()
}

func (h *harness) expectGrants() {
	h.grants.EXPECT().
		Issue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, grant entity.PreviewGrant, _ time.Duration,
		) (string, error) {
			token := uuid.NewString()
			h.issued[token] = grant

			return token, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		Read(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token string) (entity.PreviewGrant, error) {
			grant, known := h.issued[token]
			if !known {
				return entity.PreviewGrant{}, entity.ErrPreviewGrantNotFound
			}

			return grant, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		IssueTicket(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, grant entity.PreviewGrant, _ time.Duration,
		) (string, error) {
			ticket := uuid.NewString()
			h.tickets[ticket] = grant

			return ticket, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		RedeemTicket(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ticket string) (entity.PreviewGrant, error) {
			grant, known := h.tickets[ticket]
			if !known {
				return entity.PreviewGrant{}, entity.ErrPreviewGrantNotFound
			}

			delete(h.tickets, ticket)

			return grant, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		RevokeLink(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, linkID uuid.UUID) error {
			h.shutOut = append(h.shutOut, linkID)

			for token, grant := range h.issued {
				if grant.LinkID == linkID {
					delete(h.issued, token)
				}
			}

			return nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		FirstLook(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, viewer string, _ time.Duration) (bool, error) {
			if h.looks[viewer] {
				return false, nil
			}

			h.looks[viewer] = true

			return true, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		Attempt(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, subject string, _ time.Duration) (int, error) {
			h.attempts[subject]++

			return h.attempts[subject], nil
		}).
		AnyTimes()
}

func (h *harness) reported(t *testing.T, name, state string) entity.PreviewSession {
	t.Helper()

	if err := h.service.Reported(
		context.Background(), h.runner, previewMessage(name, name, state),
	); err != nil {
		t.Fatalf("report the %s preview as %s: %v", name, state, err)
	}

	return h.stored[name]
}

func (h *harness) shared(t *testing.T, name, passcode string) service.PreviewShareMinted {
	t.Helper()

	minted, err := h.service.Share(
		signedIn(context.Background(), h.caller),
		h.workspaceID,
		h.execution.ID,
		service.PreviewShareRequest{Name: name, Passcode: passcode},
	)
	if err != nil {
		t.Fatalf("mint a share link for %s: %v", name, err)
	}

	return minted
}

func (h *harness) previewEvents() []entity.ExecutionEvent {
	seen := make([]entity.ExecutionEvent, 0, len(h.recorded))

	for _, event := range h.recorded {
		if event.Kind == entity.ExecutionEventPreview {
			seen = append(seen, event)
		}
	}

	return seen
}

func (h *harness) auditedFor(action entity.AuditAction) int {
	counted := 0

	for _, entry := range h.audited {
		if entry.Action == action {
			counted++
		}
	}

	return counted
}

func signedIn(ctx context.Context, accountID uuid.UUID) context.Context {
	return identity.WithActor(ctx, entity.Actor{
		Kind:      entity.ActorKindUser,
		AccountID: accountID,
	})
}

func previewMessage(name, service, state string) entity.ChannelMessage {
	payload, err := json.Marshal(channelv1.Preview{
		Name:     name,
		Service:  service,
		State:    state,
		Occurred: time.Now().UTC(),
	})
	if err != nil {
		panic(err)
	}

	message, err := channelv1.NewRunnerMessage(
		entity.ChannelPreviewState, "exec-01ABC", payload, time.Now().UTC(),
	)
	if err != nil {
		panic(err)
	}

	return message
}

func viewerFrom(address string) entity.SessionClient {
	return entity.SessionClient{IP: netip.MustParseAddr(address), UserAgent: "Firefox"}
}

func hostFor(name string) string {
	return strings.ToLower(name + "-exec-01ABC." + previewDomain)
}

func tokenFrom(t *testing.T, url string) string {
	t.Helper()

	marker := entity.PreviewSharePath

	at := strings.Index(url, marker)
	if at < 0 {
		t.Fatalf("the share url %q carries no token", url)
	}

	return url[at+len(marker):]
}

func ticketFrom(t *testing.T, redirect string) string {
	t.Helper()

	parsed, err := neturl.Parse(redirect)
	if err != nil {
		t.Fatalf("parse the redirect %q: %v", redirect, err)
	}

	ticket := parsed.Query().Get("ticket")
	if ticket == "" {
		t.Fatalf("the redirect %q carries no ticket", redirect)
	}

	return ticket
}

func refusedWith(t *testing.T, err, wanted error) {
	t.Helper()

	if !errors.Is(err, wanted) {
		t.Fatalf("the request was refused with %v, want %v", err, wanted)
	}
}
