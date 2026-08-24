package channelv1

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	PreviewBySubdomain = "subdomain"
	PreviewByPath      = "path"

	PreviewLabelMax = 63
)

func PreviewHost(reference, title, executionID string, port int, mode, domain string) string {
	if domain == "" {
		return ""
	}

	return strings.ToLower(
		previewLabel(reference, title, executionID, port, mode) + "." + domain,
	)
}

func Slug(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	var builder strings.Builder

	dashed := false

	for _, symbol := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(symbol) || unicode.IsDigit(symbol):
			builder.WriteRune(symbol)

			dashed = false
		case !dashed && builder.Len() > 0:
			builder.WriteRune('-')

			dashed = true
		}
	}

	return strings.Trim(cutAtRuneBoundary(builder.String(), limit), "-")
}

func previewLabel(reference, title, executionID string, port int, mode string) string {
	held := Slug(reference, PreviewLabelMax)
	run := strings.ToLower(strings.TrimSpace(executionID))

	if mode != PreviewByPath {
		run = previewJoin(run, strconv.Itoa(port))
	}

	return previewJoin(held, Slug(title, previewTitleRoom(held, run)), run)
}

func previewTitleRoom(held, run string) int {
	spent := len(previewJoin(held, run))
	if spent == 0 {
		return PreviewLabelMax
	}

	return PreviewLabelMax - spent - 1
}

func previewJoin(parts ...string) string {
	held := make([]string, 0, len(parts))

	for _, part := range parts {
		if part != "" {
			held = append(held, part)
		}
	}

	return strings.Join(held, "-")
}

func cutAtRuneBoundary(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}

	return value[:limit]
}
