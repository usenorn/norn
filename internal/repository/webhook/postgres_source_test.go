package webhook

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository"
)

func packageSource(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package: %v", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "mock_") {
			continue
		}

		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		combined.Write(body)
	}

	if combined.Len() == 0 {
		t.Fatal("this guard found no source, so it is protecting nothing")
	}

	return combined.String()
}

func TestNothingRewritesARecordedAttempt(t *testing.T) {
	for _, forbidden := range []string{
		`(?is)update\s+webhook_attempts`,
		`(?is)delete\s+from\s+webhook_attempts`,
	} {
		if regexp.MustCompile(forbidden).MatchString(packageSource(t)) {
			t.Errorf(
				"this package matches %q. An attempt is what actually happened on the wire, and a "+
					"delivery log that can be edited afterwards is worth nothing to whoever is "+
					"debugging their receiver. Attempts leave only by cascade when their delivery does.",
				forbidden,
			)
		}
	}
}

func TestTheOnlyDeletesAreTheRetentionSweepAndRemovingASubscription(t *testing.T) {
	source := packageSource(t)

	deletes := regexp.MustCompile(`(?is)delete\s+from\s+(webhook_deliveries|webhook_outbox|workspace_webhooks)`).
		FindAllString(source, -1)

	if len(deletes) != 3 {
		t.Fatalf(
			"found %d deletes across the webhook tables, want exactly three: the two retention "+
				"sweeps and removing a subscription. Anything else is a way to lose history quietly.",
			len(deletes),
		)
	}

	for name, query := range map[string]string{
		"dropDeliveriesQuery": dropDeliveriesQuery,
		"dropOutboxQuery":     dropOutboxQuery,
	} {
		for _, column := range []string{"webhook_id", "state", "event", "workspace_id", "attempt"} {
			if strings.Contains(query, column) {
				t.Errorf(
					"%s filters on %s. Retention ages rows out and nothing else; a sweep that can "+
						"pick which deliveries to forget is a sweep that can be used to hide one.",
					name, column,
				)
			}
		}
	}

	if !strings.Contains(dropDeliveriesQuery, "created_at <") {
		t.Error("dropDeliveriesQuery does not filter on age")
	}

	if !strings.Contains(dropOutboxQuery, "occurred_at <") {
		t.Error("dropOutboxQuery does not filter on age")
	}
}

func TestASecretIsOnlyEverReadThroughTheCrypter(t *testing.T) {
	source := packageSource(t)

	reading := regexp.MustCompile(`(?i)secret_sealed`).FindAllString(source, -1)

	if len(reading) == 0 {
		t.Fatal("no query mentions secret_sealed, so this guard is protecting nothing")
	}

	if !strings.Contains(source, "r.crypter.Seal") || !strings.Contains(source, "r.crypter.Open") {
		t.Fatal(
			"the sealed columns are handled without the crypter. A signing secret has to be " +
				"recoverable to compute an HMAC, which is exactly why it must never sit in the " +
				"database as plaintext.",
		)
	}

	if strings.Contains(webhookColumns, "secret_sealed") {
		t.Error(
			"the ordinary read selects the sealed secret. Every list and detail path would then " +
				"carry signing material it has no use for.",
		)
	}
}

func TestNoGeneratedModelOffersAWayToRewriteTheseTables(t *testing.T) {
	entries, err := os.ReadDir("../../db/postgres")
	if err != nil {
		t.Fatalf("read generated models: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name()), "webhook") {
			t.Errorf(
				"a generated model exists at %s. These tables are blacklisted in sqlboiler.toml so "+
					"raw SQL in this package is the only surface, which is what the guards above "+
					"are able to cover.",
				entry.Name(),
			)
		}
	}
}

func TestTheDeliveryInsertWritesEveryColumnItNames(t *testing.T) {
	for name, query := range map[string]string{
		"queueDeliveryQuery": queueDeliveryQuery,
		"recordAttemptQuery": recordAttemptQuery,
		"recordOutboxQuery":  recordOutboxQuery,
		"insertWebhookQuery": insertWebhookQuery,
	} {
		columns := regexp.MustCompile(`(?s)\((.*?)\)\s*\nVALUES`).FindStringSubmatch(query)
		if columns == nil {
			columns = regexp.MustCompile(`(?s)\((.*?)\)\s*VALUES`).FindStringSubmatch(query)
		}

		if columns == nil {
			t.Fatalf("%s has no column list", name)
		}

		values := regexp.MustCompile(`VALUES \((.*?)\)`).FindStringSubmatch(query)
		if values == nil {
			t.Fatalf("%s has no VALUES list", name)
		}

		named := strings.Count(columns[1], ",") + 1
		placeholders := strings.Count(values[1], "$")

		if named != placeholders {
			t.Errorf(
				"%s names %d columns but binds %d values. A column added to the table and to the "+
					"entity but not here is written as its default, so the field silently never "+
					"persists and nothing fails loudly.",
				name, named, placeholders,
			)
		}
	}
}

func TestTheWebhookSurfaceCannotCountAnything(t *testing.T) {
	forbidden := []string{"count", "seat", "quota", "usage", "meter", "bill", "tier", "plan"}

	for _, surface := range []reflect.Type{
		reflect.TypeOf((*repository.Webhook)(nil)).Elem(),
		reflect.TypeOf((*repository.WebhookDelivery)(nil)).Elem(),
		reflect.TypeOf((*repository.WebhookOutbox)(nil)).Elem(),
	} {
		for i := range surface.NumMethod() {
			name := strings.ToLower(surface.Method(i).Name)

			for _, word := range forbidden {
				if strings.Contains(name, word) {
					t.Errorf(
						"%s exposes %q. Webhooks are free forever on every tier including "+
							"self-hosted with no licence, so nothing here may learn to count.",
						surface.Name(), surface.Method(i).Name,
					)
				}
			}
		}
	}
}
