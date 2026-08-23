package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=previews.go -destination=preview/mock_previews.go -package=preview -mock_names=Previews=MockPreviews

type PreviewDetail struct {
	Preview entity.PreviewSession
	URL     string
	Links   []entity.PreviewShareLink
}

type PreviewShareMinted struct {
	Link entity.PreviewShareLink
	URL  string
}

type PreviewShareRequest struct {
	Name     string
	Lifetime time.Duration
	Passcode string
}

type Previews interface {
	Reported(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error

	ForExecution(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
	) ([]PreviewDetail, error)
	Share(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		request PreviewShareRequest,
	) (PreviewShareMinted, error)
	RevokeShare(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID, name string,
		linkID uuid.UUID,
	) error

	Authorize(ctx context.Context, host, returnTo string) (entity.PreviewAccess, error)
	Redeem(ctx context.Context, host, token, passcode string) (entity.PreviewAccess, error)
	RedeemTicket(ctx context.Context, ticket string) (entity.PreviewAccess, error)
	Introspect(
		ctx context.Context,
		host, grant string,
		client entity.SessionClient,
	) (entity.PreviewAccess, error)
}
