package channelv1

import (
	"slices"
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

	return strings.ToLower(previewLabel(reference, title, executionID, port, mode) + "." + domain)
}

func previewLabel(reference, title, executionID string, port int, mode string) string {
	fixed := []string{executionID}
	if mode != PreviewByPath {
		fixed = append(fixed, strconv.Itoa(port))
	}

	elastic := make([]string, 0, 2)

	for _, value := range []string{reference, title} {
		spare := PreviewLabelMax - len(strings.Join(slices.Concat(elastic, fixed), "-")) - 1

		if named := Slug(value, spare); named != "" {
			elastic = append(elastic, named)
		}
	}

	return strings.Join(slices.Concat(elastic, fixed), "-")
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

func cutAtRuneBoundary(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}

	return value[:limit]
}
