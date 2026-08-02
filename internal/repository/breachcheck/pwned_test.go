package breachcheck_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/pwned"
	"github.com/usenorn/norn/internal/repository/breachcheck"
)

const knownPassword = "password"

func newRepository(t *testing.T, enabled bool, handler http.HandlerFunc) (repository, *int) {
	t.Helper()

	calls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client := pwned.New(config.Password{
		BreachCheckEnabled:  enabled,
		BreachCheckEndpoint: server.URL,
		BreachCheckTimeout:  5 * time.Second,
	})

	return breachcheck.New(client), &calls
}

type repository interface {
	Compromised(ctx context.Context, password string) (bool, error)
}

func TestARangeResponseContainingTheSuffixMarksThePasswordCompromised(t *testing.T) {
	_, suffix := entity.PasswordBreachDigest(knownPassword)

	repo, _ := newRepository(t, true, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "0000000000000000000000000000000000A:1\r\n%s:9659365\r\n", suffix)
	})

	compromised, err := repo.Compromised(context.Background(), knownPassword)
	if err != nil {
		t.Fatalf("Compromised: %v", err)
	}

	if !compromised {
		t.Fatal("a password whose suffix is in the range was not reported compromised")
	}
}

func TestARangeResponseWithoutTheSuffixLeavesThePasswordUsable(t *testing.T) {
	repo, _ := newRepository(t, true, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "0000000000000000000000000000000000A:1\r\n0000000000000000000000000000000000B:2\r\n")
	})

	compromised, err := repo.Compromised(context.Background(), knownPassword)
	if err != nil {
		t.Fatalf("Compromised: %v", err)
	}

	if compromised {
		t.Fatal("a password absent from the range was reported compromised")
	}
}

func TestTheRangeRequestCarriesOnlyTheFirstFiveDigestCharacters(t *testing.T) {
	prefix, suffix := entity.PasswordBreachDigest(knownPassword)

	var requested string

	repo, _ := newRepository(t, true, func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		_, _ = fmt.Fprint(w, "0000000000000000000000000000000000A:1\r\n")
	})

	if _, err := repo.Compromised(context.Background(), knownPassword); err != nil {
		t.Fatalf("Compromised: %v", err)
	}

	if requested != "/"+prefix {
		t.Fatalf("requested path = %q, want /%s", requested, prefix)
	}

	if requested == "/"+prefix+suffix {
		t.Fatal("the request leaked the full digest")
	}
}

func TestAFailedRangeRequestIsReportedAsAnUnavailableBreachCheck(t *testing.T) {
	repo, _ := newRepository(t, true, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := repo.Compromised(context.Background(), knownPassword); !errors.Is(err, entity.ErrPasswordBreachCheckUnavailable) {
		t.Fatalf("Compromised error = %v, want ErrPasswordBreachCheckUnavailable", err)
	}
}

func TestTheBreachCheckIsSkippedEntirelyWhenDisabled(t *testing.T) {
	repo, calls := newRepository(t, false, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	compromised, err := repo.Compromised(context.Background(), knownPassword)
	if err != nil {
		t.Fatalf("Compromised: %v", err)
	}

	if compromised {
		t.Fatal("a disabled breach check reported a password compromised")
	}

	if *calls != 0 {
		t.Fatalf("a disabled breach check made %d requests, want 0", *calls)
	}
}
