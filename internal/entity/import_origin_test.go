package entity_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnOriginNobodyConstructedClaimsNothing(t *testing.T) {
	if (entity.ImportOrigin{}).Attributed() {
		t.Fatal(
			"a zero ImportOrigin reports itself attributed, so every create path that forgot to " +
				"pass one would stamp the zero time instead of the clock",
		)
	}
}

func TestAnOriginDecodedFromARequestBodyCannotAttributeAnything(t *testing.T) {
	author := uuid.New()

	body := `{
		"CreatedAt": "2019-04-02T09:15:00Z",
		"UpdatedAt": "2019-04-02T09:15:00Z",
		"AuthorAccountID": "` + author.String() + `"
	}`

	var forged entity.ImportOrigin

	if err := json.Unmarshal([]byte(body), &forged); err != nil {
		t.Fatalf("decode origin: %v", err)
	}

	if forged.CreatedAt.IsZero() || forged.AuthorAccountID != author {
		t.Fatal("the decode did not reach the exported fields, so this test proves nothing")
	}

	if forged.Attributed() || forged.Attributes() {
		t.Fatal(
			"an origin filled by encoding/json counts as attributed. Any caller could then post " +
				"a comment dated before the workspace existed under somebody else's account.",
		)
	}

	now := time.Now().UTC()
	fallback := uuid.New()

	createdAt, updatedAt := entity.OriginStamp(&forged, now)

	if !createdAt.Equal(now) || !updatedAt.Equal(now) {
		t.Fatalf("stamp = (%v, %v), want the clock %v for a decoded origin", createdAt, updatedAt, now)
	}

	if got := entity.OriginAuthor(&forged, fallback); got != fallback {
		t.Fatalf("author = %v, want the acting account %v for a decoded origin", got, fallback)
	}
}

func TestAnOriginWithoutACreationDateIsNoOrigin(t *testing.T) {
	origin := entity.NewImportOrigin(time.Time{}, time.Now().UTC(), uuid.New())

	if origin.Attributed() {
		t.Fatal(
			"an origin whose source had no creation date is attributed, so the row would carry " +
				"the zero time rather than the moment the import ran",
		)
	}
}

func TestAnOriginNeverReportsBeingUpdatedBeforeItWasCreated(t *testing.T) {
	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(-time.Hour)

	origin := entity.NewImportOrigin(createdAt, updatedAt, uuid.New())

	gotCreated, gotUpdated := origin.Stamp(time.Now().UTC())

	if !gotCreated.Equal(createdAt) {
		t.Fatalf("created = %v, want %v", gotCreated, createdAt)
	}

	if !gotUpdated.Equal(createdAt) {
		t.Fatalf(
			"updated = %v, want it clamped up to %v. A row updated before it existed fails the "+
				"table's own ordering assumptions and sorts ahead of its own creation.",
			gotUpdated, createdAt,
		)
	}
}

func TestNoOriginAtAllLeavesTheClockAndTheActorInCharge(t *testing.T) {
	now := time.Now().UTC()
	fallback := uuid.New()

	createdAt, updatedAt := entity.OriginStamp(nil, now)

	if !createdAt.Equal(now) || !updatedAt.Equal(now) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", createdAt, updatedAt, now, now)
	}

	if got := entity.OriginAuthor(nil, fallback); got != fallback {
		t.Fatalf("author = %v, want %v", got, fallback)
	}
}

func TestAnAttributedOriginWithoutAnAuthorStillKeepsItsDates(t *testing.T) {
	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(48 * time.Hour)

	origin := entity.NewImportOrigin(createdAt, updatedAt, uuid.Nil)
	fallback := uuid.New()

	gotCreated, gotUpdated := entity.OriginStamp(&origin, time.Now().UTC())

	if !gotCreated.Equal(createdAt) || !gotUpdated.Equal(updatedAt) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", gotCreated, gotUpdated, createdAt, updatedAt)
	}

	if got := entity.OriginAuthor(&origin, fallback); got != fallback {
		t.Fatalf(
			"author = %v, want the acting account %v. An unmatched source author has no account "+
				"to point at, and created_by_account_id references one.",
			got, fallback,
		)
	}
}
