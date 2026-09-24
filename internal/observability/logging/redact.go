package logging

import (
	"context"
	"log/slog"
	"strings"
)

const Redacted = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"access_key_id":         {},
	"access_token":          {},
	"access_token_sealed":   {},
	"api_key":               {},
	"api_key_sealed":        {},
	"authorization":         {},
	"client_secret":         {},
	"client_secret_sealed":  {},
	"code":                  {},
	"code_verifier":         {},
	"cookie":                {},
	"credentials":           {},
	"dsn":                   {},
	"encryption_key":        {},
	"github_token":          {},
	"headers":               {},
	"id_token":              {},
	"password":              {},
	"password_hash":         {},
	"passwd":                {},
	"private_key":           {},
	"oauth_client_secret":   {},
	"refresh_token":         {},
	"refresh_token_sealed":  {},
	"secret":                {},
	"secret_access_key":     {},
	"secrets_sealed":        {},
	"session_token":         {},
	"set_cookie":            {},
	"token":                 {},
	"token_hash":            {},
	"token_sealed":          {},
	"verifier":              {},
	"webhook_secret_sealed": {},
}

type redactHandler struct {
	inner slog.Handler
}

func (h redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h redactHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)

	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))

		return true
	})

	return h.inner.Handle(ctx, redacted)
}

func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		redacted[i] = redactAttr(attr)
	}

	return redactHandler{inner: h.inner.WithAttrs(redacted)}
}

func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{inner: h.inner.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()

	if _, sensitive := sensitiveKeys[strings.ToLower(attr.Key)]; sensitive {
		attr.Value = slog.StringValue(Redacted)

		return attr
	}

	if attr.Value.Kind() != slog.KindGroup {
		return attr
	}

	group := attr.Value.Group()

	redacted := make([]slog.Attr, len(group))
	for i, nested := range group {
		redacted[i] = redactAttr(nested)
	}

	attr.Value = slog.GroupValue(redacted...)

	return attr
}
