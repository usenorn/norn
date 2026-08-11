package entity

import (
	"regexp"
	"strings"
)

const EvidenceRedacted = "[REDACTED]"

type evidenceRedaction struct {
	expression  *regexp.Regexp
	replacement string
}

var evidenceRedactions = []evidenceRedaction{
	{
		regexp.MustCompile(
			`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(-----END [A-Z ]*PRIVATE KEY-----|\z)`,
		),
		EvidenceRedacted,
	},
	{
		regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{20,}|` +
			`glpat-[A-Za-z0-9_\-]{16,}|xox[abposr]-[A-Za-z0-9\-]{10,}|` +
			`sk-(?:ant-)?[A-Za-z0-9_\-]{20,}|[sr]k_(?:live|test)_[A-Za-z0-9]{16,}|` +
			`AIza[0-9A-Za-z_\-]{35}|npm_[A-Za-z0-9]{36}|A(?:KIA|SIA|ROA|IDA|NPA)[0-9A-Z]{16})\b`),
		EvidenceRedacted,
	},
	{
		regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}`),
		EvidenceRedacted,
	},
	{
		regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://[^\s:/@]+):[^\s/@]+@`),
		`${1}:` + EvidenceRedacted + `@`,
	},
	{
		regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._\-+/=]{12,}`),
		`${1} ` + EvidenceRedacted,
	},
	{
		regexp.MustCompile(`(?im)([\w.\-]*(?:password|passwd|secret|token|api[_\-]?key|` +
			`access[_\-]?key|private[_\-]?key|credential|authorization|dsn|` +
			`connection[_\-]?string)[\w.\-]*)(\s*[:=]\s*)((?:bearer|basic|token)\s+)?` +
			`(\[REDACTED\]|"[^"\n]*"|'[^'\n]*'|[^\s,;}\]]+)`),
		`${1}${2}` + EvidenceRedacted,
	},
}

func RedactEvidence(text string) (string, int) {
	for _, redaction := range evidenceRedactions {
		text = redaction.expression.ReplaceAllString(text, redaction.replacement)
	}

	return text, strings.Count(text, EvidenceRedacted)
}
