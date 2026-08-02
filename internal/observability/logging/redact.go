package logging

import (
	"context"
	"log/slog"
	"strings"
)

const redactedPlaceholder = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"access_key_id":     {},
	"api_key":           {},
	"authorization":     {},
	"cookie":            {},
	"credentials":       {},
	"dsn":               {},
	"password":          {},
	"password_hash":     {},
	"passwd":            {},
	"private_key":       {},
	"refresh_token":     {},
	"secret":            {},
	"secret_access_key": {},
	"session_token":     {},
	"set_cookie":        {},
	"token":             {},
	"token_hash":        {},
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
		attr.Value = slog.StringValue(redactedPlaceholder)

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
