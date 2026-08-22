package channelv1

import (
	"strings"

	"golang.org/x/mod/semver"
)

const (
	MinimumRunner = "0.1.0"
	InstallRunner = "curl -fsSL https://get.norn.so/runner | bash"
)

func Released(version string) bool {
	return semver.IsValid(canonicalise(version))
}

func RunnerSupported(reported, minimum string) bool {
	if !Released(reported) || !Released(minimum) {
		return true
	}

	return semver.Compare(canonicalise(reported), canonicalise(minimum)) >= 0
}

func canonicalise(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}

	if !strings.HasPrefix(trimmed, "v") {
		return "v" + trimmed
	}

	return trimmed
}
