package entity_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestASignInCodeIsSixDigitsAndDiffersBetweenSignIns(t *testing.T) {
	shape := regexp.MustCompile(`^[0-9]{6}$`)
	seen := make(map[string]bool, 64)

	for range 64 {
		code, hash, err := entity.NewSignInCode()
		if err != nil {
			t.Fatalf("NewSignInCode: %v", err)
		}

		if !shape.MatchString(code) {
			t.Fatalf("code = %q, want six digits. Anything else cannot be read off a phone.", code)
		}

		if !bytes.Equal(hash, entity.HashSignInCode(code)) {
			t.Fatal("the returned hash is not the hash of the returned code")
		}

		seen[code] = true
	}

	if len(seen) < 32 {
		t.Fatalf(
			"64 draws produced %d distinct codes. A code somebody can guess from the last one "+
				"is not a second factor.",
			len(seen),
		)
	}
}

func TestACodeIsReadTheWayPeopleTypeIt(t *testing.T) {
	challenge := entity.SignInChallenge{CodeHash: entity.HashSignInCode("012345")}

	for _, typed := range []string{"012345", " 012345 ", "012-345", "012 345"} {
		if !challenge.Answers(typed) {
			t.Errorf("%q was refused; it is the same code with the spacing people paste", typed)
		}
	}

	for _, wrong := range []string{"012346", "12345", "", "01234a"} {
		if challenge.Answers(wrong) {
			t.Errorf("%q was accepted", wrong)
		}
	}
}

func TestAChallengeStopsAnsweringOnceItLapsesOrIsGuessedAt(t *testing.T) {
	issued := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	challenge := entity.SignInChallenge{
		IssuedAt:  issued,
		ExpiresAt: issued.Add(entity.SignInChallengeTTL),
	}

	if challenge.ExpiredAt(issued.Add(entity.SignInChallengeTTL - time.Second)) {
		t.Error("a challenge lapsed before its own deadline")
	}

	if !challenge.ExpiredAt(issued.Add(entity.SignInChallengeTTL)) {
		t.Error("a challenge outlived its deadline")
	}

	if challenge.Exhausted() {
		t.Error("a fresh challenge reported that its attempts were spent")
	}

	if challenge.AttemptsLeft() != entity.SignInChallengeMaxAttempts {
		t.Errorf("attempts left = %d, want %d", challenge.AttemptsLeft(), entity.SignInChallengeMaxAttempts)
	}

	challenge.Attempts = entity.SignInChallengeMaxAttempts

	if !challenge.Exhausted() {
		t.Fatal(
			"a challenge that has taken every attempt still accepts another. Six digits fall to " +
				"guessing quickly once nothing counts them.",
		)
	}

	if challenge.AttemptsLeft() != 0 {
		t.Errorf("attempts left = %d, want 0", challenge.AttemptsLeft())
	}
}
