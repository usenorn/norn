package imports

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
)

func sealingRuns(t *testing.T, key string) *runRepository {
	t.Helper()

	sealer, err := crypter.New(config.Security{EncryptionKey: key})
	if err != nil {
		t.Fatalf("build crypter: %v", err)
	}

	return &runRepository{crypter: sealer}
}

func TestAStoredSourceKeyIsCiphertextRatherThanTheKeyItself(t *testing.T) {
	repository := sealingRuns(t, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, crypter.KeyBytes)))

	const secret = "lin_api_01HZXKQ9V4T7"

	sealed, err := repository.seal(secret)
	if err != nil {
		t.Fatalf("seal source key: %v", err)
	}

	if bytes.Contains(sealed, []byte(secret)) {
		t.Fatalf(
			"what would be written to source_secret_sealed contains the key verbatim. A source key "+
				"reads every issue in somebody else's tracker, and a backup, a replica or a support "+
				"session with a psql prompt is enough to lift it if the column is plaintext. Stored "+
				"bytes: %q",
			sealed,
		)
	}

	opened, err := repository.open(sealed)
	if err != nil {
		t.Fatalf("open source key: %v", err)
	}

	if opened != secret {
		t.Fatalf(
			"the key opened as %q, want %q. It has to come back exactly as it went in or the "+
				"staging pass authenticates with something the source has never seen.",
			opened, secret,
		)
	}
}

func TestAnInstanceWithNoEncryptionKeyRefusesBySayingSo(t *testing.T) {
	repository := sealingRuns(t, "")

	if _, err := repository.seal("lin_api_01HZXKQ9V4T7"); !errors.Is(err, entity.ErrImportEncryptionKeyMissing) {
		t.Errorf(
			"sealing on an instance with no key returned %v. The crypter's own error names no "+
				"domain, so a caller could only report it as an unexplained failure rather than as "+
				"the one thing an operator has to go and configure.",
			err,
		)
	}

	if _, err := repository.open([]byte("sealed")); !errors.Is(err, entity.ErrImportEncryptionKeyMissing) {
		t.Errorf("opening on an instance with no key returned %v, want the domain error", err)
	}
}

func TestTheOrdinaryReadOfARunNeverSelectsTheSealedKey(t *testing.T) {
	if strings.Contains(runColumns, "source_secret_sealed IS NOT NULL") &&
		strings.Count(runColumns, "source_secret_sealed") != 1 {
		t.Error("runColumns names the sealed column more than once")
	}

	for _, line := range strings.Split(runColumns, "\n") {
		if strings.Contains(line, "source_secret_sealed") && !strings.Contains(line, "IS NOT NULL") {
			t.Errorf(
				"the ordinary read selects the sealed key in %q. Every read of a run — the wizard, "+
					"the report, the rescue sweep — would then carry a credential it has no use for, "+
					"and a run only ever needs to know that a key is stored.",
				strings.TrimSpace(line),
			)
		}
	}

	if !strings.Contains(sourceConfigQuery, "source_secret_sealed") {
		t.Error("nothing selects the sealed key at all, so staging could never authenticate")
	}
}

func TestSavingAConfigurationWithNoKeyLeavesTheStoredOneWhereItIs(t *testing.T) {
	if !strings.Contains(saveSourceConfigQuery, "coalesce($2, source_secret_sealed)") {
		t.Fatal(
			"saving a configuration writes the sealed column unconditionally. The key is never read " +
				"back to whoever is configuring the run, so returning to change a team selection " +
				"would blank the key they entered before and strand the staging pass with nothing to " +
				"authenticate with.",
		)
	}
}
