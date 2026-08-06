package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var errRefused = errors.New("the row was refused")

func TestOutsideATransactionASavepointIsJustTheWorkItself(t *testing.T) {
	client := &Client{}
	ran := false

	if err := client.WithSavepoint(context.Background(), func(context.Context) error {
		ran = true

		return nil
	}); err != nil {
		t.Fatalf("running outside a transaction: %v", err)
	}

	if !ran {
		t.Fatal(
			"the work never ran. A savepoint is a point inside a transaction and there is nothing " +
				"to take one against out here, so the honest shape is to run what was asked and " +
				"leave the caller's code identical either way.",
		)
	}
}

func TestOutsideATransactionAFailureIsReturnedUntouched(t *testing.T) {
	client := &Client{}

	err := client.WithSavepoint(context.Background(), func(context.Context) error {
		return errRefused
	})

	if !errors.Is(err, errRefused) {
		t.Fatalf(
			"the refusal came back as %v. Callers read what came out of the work to decide whether "+
				"the row is an outcome or an error, and a savepoint that reshaped it would change "+
				"that decision by where it was called from.",
			err,
		)
	}
}

func TestTwoSavepointsNeverShareAName(t *testing.T) {
	seen := map[string]bool{}

	for range 1000 {
		name := savepointName()

		if seen[name] {
			t.Fatalf(
				"%s was drawn twice. A savepoint reusing an open name displaces it, so rolling "+
					"back to it would unwind past whatever took it first — every row a nested "+
					"caller had already released.",
				name,
			)
		}

		if strings.ContainsAny(name, "-\"; ") {
			t.Fatalf(
				"%q is not a bare identifier. A savepoint name cannot be parameterized and is "+
					"concatenated into the statement as it stands.",
				name,
			)
		}

		seen[name] = true
	}
}
