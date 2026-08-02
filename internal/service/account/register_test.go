package account_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const password = "correct horse battery staple"

func TestRegisterWithoutAPasswordProducesAnAccountWithNoHash(t *testing.T) {
	h := newHarness(t)

	var captured entity.Account

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account
			account.ID = uuid.New()

			return account, nil
		})

	if _, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if captured.PasswordHash != "" {
		t.Fatal("an account registered without a password must have no hash")
	}

	if captured.Status != entity.AccountStatusActive {
		t.Fatalf("status = %q, want active", captured.Status)
	}
}

func TestRegisterStoresAnArgon2idHashAndNeverThePlaintext(t *testing.T) {
	h := newHarness(t)

	var captured entity.Account

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	if _, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
		Password:    password,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if !strings.HasPrefix(captured.PasswordHash, "$argon2id$") {
		t.Fatalf("stored hash %q is not argon2id", captured.PasswordHash)
	}

	if strings.Contains(captured.PasswordHash, password) {
		t.Fatal("stored hash contains the plaintext password")
	}

	matches, err := entity.VerifyPassword(captured.PasswordHash, password)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !matches {
		t.Fatal("the stored hash does not verify the registered password")
	}
}

func TestRegisterNormalizesTheEmailAndAppliesTheDefaultTimezone(t *testing.T) {
	h := newHarness(t)

	var captured entity.Account

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	if _, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "  Ada@Example.COM ",
		DisplayName: "Ada Lovelace",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if captured.Email != "ada@example.com" {
		t.Fatalf("stored email = %q, want %q", captured.Email, "ada@example.com")
	}

	if captured.Timezone != entity.DefaultTimezone {
		t.Fatalf("timezone = %q, want %q", captured.Timezone, entity.DefaultTimezone)
	}
}

func TestRegisterConsultsNoLimitBeforeCreating(t *testing.T) {
	h := newHarness(t)

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	if _, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestRegisterPassesTheEmailTakenSentinelThroughUnwrapped(t *testing.T) {
	h := newHarness(t)

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.Account{}, entity.ErrAccountEmailTaken)

	_, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
	})
	if !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("Register error = %v, want ErrAccountEmailTaken", err)
	}
}

func TestRegisterReportsEveryInvalidFieldAtOnce(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Register(context.Background(), service.RegisterAccountInput{
		Email:       "not-an-address",
		DisplayName: "",
		Timezone:    "Mars/Olympus",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Register error = %v, want a ValidationError", err)
	}

	codes := map[string]string{}
	for _, field := range validation.Fields {
		codes[field.Field] = field.Code
	}

	want := map[string]string{
		"email":        entity.ValidationCodeMalformed,
		"display_name": entity.ValidationCodeRequired,
		"timezone":     entity.ValidationCodeUnknownTimezone,
	}

	for field, code := range want {
		if codes[field] != code {
			t.Errorf("field %q code = %q, want %q", field, codes[field], code)
		}
	}
}

func TestNoLayerExposesAnAccountCountingOperation(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "limit"}

	surfaces := map[string]reflect.Type{
		"repository.Account": reflect.TypeOf((*repository.Account)(nil)).Elem(),
		"service.Accounts":   reflect.TypeOf((*service.Accounts)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ToLower(surface.Method(i).Name)

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf("%s exposes %q, which counts or caps accounts", name, surface.Method(i).Name)
				}
			}
		}
	}
}
