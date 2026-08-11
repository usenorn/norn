package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestACredentialInAConnectionStringNeverReachesStorage(t *testing.T) {
	cases := []struct {
		name string
		text string
		gone string
	}{
		{
			"postgres dsn",
			"psql postgres://norn:s3cr3t-pw@db.internal:5432/norn",
			"s3cr3t-pw",
		},
		{
			"clone url with a token",
			"git clone https://x-access-token:ghs_AbCdEf0123456789xyz@github.com/acme/web.git",
			"ghs_AbCdEf0123456789xyz",
		},
		{
			"environment dump",
			"PGPASSWORD=hunter2correct\nHOME=/root",
			"hunter2correct",
		},
		{
			"yaml secret",
			`api_key: "sk-ant-0123456789abcdefghij"`,
			"sk-ant-0123456789abcdefghij",
		},
		{
			"authorization header",
			`curl -H "Authorization: Bearer eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM"`,
			"eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM",
		},
		{
			"aws access key",
			"aws_access_key_id AKIAIOSFODNN7EXAMPLE",
			"AKIAIOSFODNN7EXAMPLE",
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			redacted, count := entity.RedactEvidence(testcase.text)

			if strings.Contains(redacted, testcase.gone) {
				t.Fatalf("redacting %q left the secret behind: %q", testcase.name, redacted)
			}

			if count == 0 {
				t.Fatalf("redacting %q reported no removals", testcase.name)
			}
		})
	}
}

func TestAPrivateKeyBlockIsRemovedWholeRatherThanLineByLine(t *testing.T) {
	key := "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEowIBAAKCAQEAtGzKq1\nrZmFuZHJvbWVkYQ==\n" +
		"-----END RSA PRIVATE KEY-----"

	redacted, count := entity.RedactEvidence("read the key:\n" + key + "\ndone")

	if strings.Contains(redacted, "rZmFuZHJvbWVkYQ") || strings.Contains(redacted, "BEGIN RSA") {
		t.Fatalf("the key survived redaction: %q", redacted)
	}

	if count != 1 {
		t.Fatalf("removed %d values, want the block counted once", count)
	}

	if !strings.Contains(redacted, "read the key:") || !strings.Contains(redacted, "done") {
		t.Fatalf("redaction ate the surrounding output: %q", redacted)
	}
}

func TestAnUnterminatedPrivateKeyIsStillRemoved(t *testing.T) {
	cut := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA"

	redacted, _ := entity.RedactEvidence(cut)

	if strings.Contains(redacted, "b3BlbnNzaC1rZXktdjEAAAAA") {
		t.Fatalf("a half-transmitted key survived redaction: %q", redacted)
	}
}

func TestARedactedLineKeepsTheShapeOfWhatWasRun(t *testing.T) {
	redacted, _ := entity.RedactEvidence("DATABASE_PASSWORD=hunter2correct")

	if !strings.HasPrefix(redacted, "DATABASE_PASSWORD=") {
		t.Fatalf("redaction lost the key it was set against: %q", redacted)
	}

	if !strings.Contains(redacted, entity.EvidenceRedacted) {
		t.Fatalf("redaction did not mark what it removed: %q", redacted)
	}
}

func TestACommitShaAndAUuidSurviveRedaction(t *testing.T) {
	verbatim := "HEAD is 9f2c1ab4d5e6f708192a3b4c5d6e7f8091a2b3c4\n" +
		"run 3f6b2a10-8c4d-4e2f-9a1b-7c5d3e2f1a0b passed in 4.21s\n" +
		"bundle sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	redacted, count := entity.RedactEvidence(verbatim)

	if redacted != verbatim {
		t.Fatalf("redaction rewrote legitimate high-entropy output:\n%q", redacted)
	}

	if count != 0 {
		t.Fatalf("redaction reported %d removals on clean output", count)
	}
}

func TestTheNumberOfRemovedValuesIsReportedRatherThanHidden(t *testing.T) {
	_, count := entity.RedactEvidence("token=abc123def456\npassword=hunter2correct")

	if count != 2 {
		t.Fatalf("reported %d removals, want 2", count)
	}
}

func TestRedactionRunsBeforeTruncationSoASecretCannotHideAcrossTheCut(t *testing.T) {
	key := "-----BEGIN RSA PRIVATE KEY-----\n" +
		strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=\n", 400) +
		"-----END RSA PRIVATE KEY-----"

	redacted, _ := entity.RedactEvidence(key)
	cut, truncated := entity.TruncateEvidenceOutput(redacted)

	if truncated {
		t.Fatal("the redacted key was still long enough to need truncating")
	}

	if strings.Contains(cut, "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo") {
		t.Fatalf("key material survived: %q", cut)
	}
}

func TestTruncationKeepsBothEndsAndNeverSplitsARune(t *testing.T) {
	output := "START" + strings.Repeat("é", entity.EvidenceOutputMaxBytes) + "END"

	cut, truncated := entity.TruncateEvidenceOutput(output)

	if !truncated {
		t.Fatal("oversized output was not truncated")
	}

	if len(cut) > entity.EvidenceOutputMaxBytes {
		t.Fatalf("truncated output is %d bytes, want at most %d", len(cut), entity.EvidenceOutputMaxBytes)
	}

	if !strings.HasPrefix(cut, "START") || !strings.HasSuffix(cut, "END") {
		t.Fatalf("truncation dropped an end of the output: %q … %q", cut[:16], cut[len(cut)-16:])
	}

	if strings.ContainsRune(cut, '�') {
		t.Fatal("truncation split a multi-byte rune")
	}
}

func TestOutputInsideTheLimitIsStoredExactlyAsItArrived(t *testing.T) {
	output := strings.Repeat("all tests passed\n", 100)

	cut, truncated := entity.TruncateEvidenceOutput(output)

	if truncated || cut != output {
		t.Fatal("output within the limit was altered")
	}
}
