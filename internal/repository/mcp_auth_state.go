package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_auth_state.go -destination=mcpauthstate/mock_mcp_auth_state.go -package=mcpauthstate -mock_names=MCPAuthState=MockMCPAuthState

type MCPAuthState interface {
	PutRequest(ctx context.Context, requestID string, request entity.MCPAuthRequest) error
	GetRequest(ctx context.Context, requestID string) (entity.MCPAuthRequest, error)
	TakeRequest(ctx context.Context, requestID string) (entity.MCPAuthRequest, error)
	PutCode(ctx context.Context, code string, grant entity.MCPAuthCode) error
	TakeCode(ctx context.Context, code string) (entity.MCPAuthCode, error)
}
