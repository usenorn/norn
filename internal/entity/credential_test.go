package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestHashPasswordProducesAVerifiableArgon2idHash(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash %q is not argon2id", hash)
	}

	if strings.Contains(hash, password) {
		t.Fatal("hash contains the plaintext password")
	}

	ok, err := entity.VerifyPassword(hash, password)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !ok {
		t.Fatal("the correct password did not verify")
	}
}

func TestHashPasswordIsSaltedPerCall(t *testing.T) {
	first, err := entity.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	second, err := entity.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if first == second {
		t.Fatal("two hashes of the same password are identical")
	}
}

func TestVerifyPasswordRejectsAWrongPassword(t *testing.T) {
	hash, err := entity.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := entity.VerifyPassword(hash, "incorrect horse battery staple")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if ok {
		t.Fatal("a wrong password verified")
	}
}

func TestVerifyPasswordRejectsAMalformedHash(t *testing.T) {
	for _, hash := range []string{"", "plaintext", "$argon2id$v=19$m=65536$abc", "$bcrypt$v=19$m=1,t=1,p=1$YWJj$YWJj"} {
		if _, err := entity.VerifyPassword(hash, "correct horse battery staple"); !errors.Is(err, entity.ErrPasswordHashMalformed) {
			t.Errorf("VerifyPassword(%q) error = %v, want ErrPasswordHashMalformed", hash, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"too short", strings.Repeat("a", entity.PasswordMinLen-1), entity.ValidationCodeTooShort},
		{"at the minimum", strings.Repeat("a", entity.PasswordMinLen), ""},
		{"too long", strings.Repeat("a", entity.PasswordMaxLen+1), entity.ValidationCodeTooLong},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidatePassword("password", c.value).Code; got != c.code {
				t.Errorf("ValidatePassword code = %q, want %q", got, c.code)
			}
		})
	}
}
