package repository

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=inboundmail.go -destination=inboundmail/mock_inboundmail.go -package=inboundmail -mock_names=InboundMail=MockInboundMail

type InboundMail interface {
	Verify(header http.Header, body []byte) (entity.InboundNotice, error)
	Fetch(ctx context.Context, messageID string) (entity.InboundMessage, error)
}
