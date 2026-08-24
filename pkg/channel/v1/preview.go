package channelv1

import (
	"strconv"
	"strings"
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
	tail := executionID
	if mode != PreviewByPath {
		tail += "-" + strconv.Itoa(port)
	}

	head := hostLabel(reference, PreviewLabelMax-len(tail)-1)
	spare := PreviewLabelMax - len(joinLabels(head, tail)) - 1

	return joinLabels(joinLabels(head, hostLabel(title, spare)), tail)
}

func joinLabels(head, tail string) string {
	if head == "" {
		return tail
	}

	if tail == "" {
		return head
	}

	return head + "-" + tail
}

func hostLabel(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	var builder strings.Builder

	dashed := false

	for _, symbol := range strings.ToLower(value) {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9':
			builder.WriteRune(symbol)

			dashed = false
		case !dashed && builder.Len() > 0:
			builder.WriteRune('-')

			dashed = true
		}
	}

	label := builder.String()
	if len(label) > limit {
		label = label[:limit]
	}

	return strings.Trim(label, "-")
}
