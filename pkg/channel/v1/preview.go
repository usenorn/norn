package channelv1

import "strings"

const (
	PreviewBySubdomain = "subdomain"
	PreviewByPath      = "path"
)

func PreviewHost(name, executionID, mode, domain string) string {
	if domain == "" {
		return ""
	}

	if mode == PreviewByPath {
		return strings.ToLower(executionID + "." + domain)
	}

	return strings.ToLower(name + "-" + executionID + "." + domain)
}
