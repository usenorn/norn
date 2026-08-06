package entity_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func declaredWebhookEvents(t *testing.T) []string {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(fileSet, "webhook.go", nil, 0)
	if err != nil {
		t.Fatalf("parse webhook.go: %v", err)
	}

	declared := make([]string, 0, 24)

	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}

		named, ok := spec.Type.(*ast.Ident)
		if !ok || named.Name != "WebhookEvent" {
			return true
		}

		for _, value := range spec.Values {
			literal, ok := value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}

			declared = append(declared, strings.Trim(literal.Value, `"`))
		}

		return true
	})

	if len(declared) == 0 {
		t.Fatal("found no WebhookEvent constants, so this guard is protecting nothing")
	}

	return declared
}

func TestEverySubscribableEventIsInTheAllowList(t *testing.T) {
	for _, event := range declaredWebhookEvents(t) {
		if event == string(entity.WebhookTest) {
			continue
		}

		if !entity.WebhookEvent(event).Valid() {
			t.Errorf(
				"the event %q is declared but missing from WebhookEvents(), so nobody can subscribe "+
					"to it and anything it fires is dropped without a trace. The list is maintained "+
					"by hand; every new event must be added twice.",
				event,
			)
		}
	}

	if len(entity.WebhookEvents()) != len(declaredWebhookEvents(t))-1 {
		t.Errorf(
			"WebhookEvents() lists %d events but %d are declared besides the test event",
			len(entity.WebhookEvents()), len(declaredWebhookEvents(t))-1,
		)
	}
}

func TestTheTestEventCannotBeSubscribedTo(t *testing.T) {
	if entity.WebhookTest.Valid() {
		t.Fatal(
			"webhook.test is subscribable. It exists so a destination can be verified on demand; " +
				"a subscription to it would deliver nothing and read as a broken integration.",
		)
	}
}

func TestEveryEventResolvesToAScopeAndAResource(t *testing.T) {
	for _, event := range entity.WebhookEvents() {
		switch entity.WebhookEventScope(event) {
		case entity.WebhookScopeTeam, entity.WebhookScopeWorkspace,
			entity.WebhookScopeAdmin, entity.WebhookScopeAdminTeam:
		default:
			t.Errorf("event %q has no scope rule, so fan-out cannot decide who may see it", event)
		}

		if resource := entity.WebhookEventResource(event); resource == "" {
			t.Errorf("event %q has no resource, so its permission cannot be resolved", event)
		}
	}
}

func TestMembershipEventsAreAdministrative(t *testing.T) {
	for _, event := range []entity.WebhookEvent{
		entity.WebhookMembershipAdded,
		entity.WebhookMembershipChanged,
		entity.WebhookMembershipRemoved,
	} {
		if entity.WebhookEventScope(event) != entity.WebhookScopeAdmin {
			t.Errorf(
				"event %q is not administrative. Who has access to a workspace is an administrative "+
					"fact, and a demoted owner must stop receiving it immediately.",
				event,
			)
		}
	}
}

func TestTheRetryLadderIsBoundedAndIncreasing(t *testing.T) {
	previous := time.Duration(0)
	total := time.Duration(0)
	delays := 0

	for attempt := 1; attempt <= entity.WebhookMaxAttempts+2; attempt++ {
		delay, again := entity.WebhookRetryDelay(attempt)

		if !again {
			if attempt <= 1 {
				t.Fatal("the first failure schedules no retry at all")
			}

			break
		}

		if delay <= previous {
			t.Errorf(
				"attempt %d waits %s, which is not longer than the %s before it. A ladder that "+
					"stops increasing hammers a struggling receiver instead of backing off.",
				attempt, delay, previous,
			)
		}

		previous = delay
		total += delay
		delays++
	}

	if delays >= entity.WebhookMaxAttempts {
		t.Errorf("the ladder offers %d delays but only %d attempts are allowed", delays, entity.WebhookMaxAttempts)
	}

	if total > 24*time.Hour {
		t.Errorf(
			"a delivery can stay in flight for %s. A receiver fixed the same day should see the "+
				"retry land, not find it the next morning.",
			total,
		)
	}
}

func TestASignatureCoversTheBodyTheTimestampAndTheDelivery(t *testing.T) {
	secret, err := entity.NewWebhookSecret()
	if err != nil {
		t.Fatalf("mint webhook secret: %v", err)
	}

	if !strings.HasPrefix(secret, entity.WebhookSecretPrefix) {
		t.Errorf("minted secret %q does not carry the webhook prefix", secret)
	}

	now := time.Now()
	delivery := uuid.New()
	body := []byte(`{"event":"issue.created"}`)

	signature := entity.SignWebhook(secret, now, delivery, body)

	if signature != entity.SignWebhook(secret, now, delivery, body) {
		t.Fatal("signing the same request twice produced different signatures")
	}

	for _, altered := range []string{
		entity.SignWebhook(secret, now.Add(time.Second), delivery, body),
		entity.SignWebhook(secret, now, uuid.New(), body),
		entity.SignWebhook(secret, now, delivery, append(body, ' ')),
		entity.SignWebhook(secret+"x", now, delivery, body),
	} {
		if altered == signature {
			t.Error(
				"changing the timestamp, the delivery, the body or the secret left the signature " +
					"unchanged, so a receiver verifying it learns nothing",
			)
		}
	}

	if !strings.HasPrefix(signature, entity.WebhookSignatureVersion+"=") {
		t.Errorf("signature %q carries no version, so a second scheme could never be added", signature)
	}
}

func TestASecretHintRevealsAlmostNothing(t *testing.T) {
	secret, err := entity.NewWebhookSecret()
	if err != nil {
		t.Fatalf("mint webhook secret: %v", err)
	}

	hint := entity.WebhookSecretHint(secret)

	if len(hint) != entity.WebhookSecretHintLen {
		t.Fatalf("hint %q is %d characters, want %d", hint, len(hint), entity.WebhookSecretHintLen)
	}

	if strings.Contains(hint, entity.WebhookSecretPrefix) {
		t.Error("the hint carries the prefix rather than the tail, which identifies nothing")
	}
}

func TestOnlyASettledDeliveryCanBeReplayed(t *testing.T) {
	for _, probe := range []struct {
		state      entity.WebhookDeliveryState
		replayable bool
	}{
		{entity.WebhookDeliveryPending, false},
		{entity.WebhookDeliverySucceeded, true},
		{entity.WebhookDeliveryFailed, true},
	} {
		delivery := entity.WebhookDelivery{State: probe.state}

		if delivery.Replayable() != probe.replayable {
			t.Errorf(
				"a %s delivery reports replayable = %v; replaying one still in flight would "+
					"deliver the same event twice for no reason",
				probe.state, delivery.Replayable(),
			)
		}
	}
}

func TestARotatedSecretSignsWithBothUntilTheGraceEnds(t *testing.T) {
	now := time.Now()
	expires := now.Add(time.Hour)

	rotating := entity.Webhook{
		Secret:          "nrnwhs_new",
		PreviousSecret:  "nrnwhs_old",
		SecretExpiresAt: &expires,
	}

	if got := rotating.SigningSecrets(now); len(got) != 2 {
		t.Fatalf(
			"a webhook inside its rotation grace signs with %d secrets, want 2. A receiver "+
				"updating its configuration would drop every delivery in between.",
			len(got),
		)
	}

	if got := rotating.SigningSecrets(expires.Add(time.Second)); len(got) != 1 {
		t.Errorf("an expired previous secret is still being used to sign, want %d secrets, got %d", 1, len(got))
	}

	fresh := entity.Webhook{Secret: "nrnwhs_only"}

	if got := fresh.SigningSecrets(now); len(got) != 1 {
		t.Errorf("a webhook that was never rotated signs with %d secrets, want 1", len(got))
	}
}

func TestADeliveryCursorSurvivesEncoding(t *testing.T) {
	original := entity.WebhookDeliveryCursor{
		CreatedAt: time.Now().UTC().Truncate(time.Nanosecond),
		ID:        uuid.New(),
	}

	decoded, err := entity.DecodeWebhookDeliveryCursor(original.Encode())
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}

	if decoded.ID != original.ID || !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("cursor round-trip changed %v into %v", original, decoded)
	}

	for _, malformed := range []string{"", "not-base64!", "c2hvcnQ"} {
		if _, err := entity.DecodeWebhookDeliveryCursor(malformed); err == nil {
			t.Errorf("the malformed cursor %q decoded without complaint", malformed)
		}
	}
}

func TestADestinationMustBeAnAbsoluteHTTPURLWithNoCredentials(t *testing.T) {
	for _, probe := range []struct {
		url   string
		valid bool
	}{
		{"https://example.com/hooks/norn", true},
		{"http://example.com/hooks", true},
		{"https://example.com/hooks?team=eng", true},
		{"", false},
		{"/hooks", false},
		{"ftp://example.com/hooks", false},
		{"https://user:pass@example.com/hooks", false},
		{"https://example.com/hooks#fragment", false},
		{"https:///hooks", false},
	} {
		failure := entity.ValidateWebhookURL("url", probe.url)

		if (failure == entity.FieldError{}) != probe.valid {
			t.Errorf("destination %q accepted = %v, want %v", probe.url, failure == entity.FieldError{}, probe.valid)
		}
	}
}

func TestEventSelectionIsBoundedAndKnown(t *testing.T) {
	if failure := entity.ValidateWebhookEvents("events", nil); failure == (entity.FieldError{}) {
		t.Error("a subscription with no events was accepted, and it could never deliver anything")
	}

	if failure := entity.ValidateWebhookEvents("events", []entity.WebhookEvent{"issue.exploded"}); failure == (entity.FieldError{}) {
		t.Error("an unknown event was accepted")
	}

	if failure := entity.ValidateWebhookEvents("events", []entity.WebhookEvent{entity.WebhookTest}); failure == (entity.FieldError{}) {
		t.Error("the test event was accepted as a subscription")
	}

	if failure := entity.ValidateWebhookEvents("events", entity.WebhookEvents()); failure != (entity.FieldError{}) {
		t.Errorf("the whole catalogue was refused: %v", failure)
	}
}
