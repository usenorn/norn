package aiprovider_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	aiprovidersvc "github.com/usenorn/norn/internal/service/aiprovider"
)

func TestOnlyAWorkspaceAdministratorReachesTheProvider(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleMember)
	h.configured(workspaceID, "sk-member-can-see", "gpt-6-luna")

	calls := map[string]func() error{
		"read":   func() error { _, err := h.service.Get(context.Background(), workspaceID); return err },
		"models": func() error { _, err := h.service.Models(context.Background(), workspaceID); return err },
		"test":   func() error { _, err := h.service.Test(context.Background(), workspaceID); return err },
		"remove": func() error { return h.service.Remove(context.Background(), workspaceID) },
		"configure": func() error {
			_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
				Provider: entity.AIProviderOpenAI, APIKey: "sk-new",
			})

			return err
		},
	}

	for name, call := range calls {
		if err := call(); !errors.Is(err, entity.ErrAccountForbidden) {
			t.Errorf("%s by a member: err = %v, want ErrAccountForbidden", name, err)
		}
	}

	if _, kept := h.store[workspaceID]; !kept {
		t.Fatal("a member's remove deleted the workspace's key")
	}
}

func TestARejectedKeyIsNeverStored(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)

	_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider: entity.AIProviderOpenAI,
		APIKey:   "sk-typo",
	})
	if !errors.Is(err, entity.ErrAIProviderKeyRejected) {
		t.Fatalf("err = %v, want ErrAIProviderKeyRejected", err)
	}

	if _, saved := h.store[workspaceID]; saved {
		t.Fatal("a key the provider refused was stored, so the workspace now holds a credential nobody can use")
	}

	if len(h.audited) != 0 {
		t.Fatalf("audited %v for a save that never happened", h.actions())
	}
}

func TestAFirstSaveProvesTheKeyAndLeavesTheConnectionUntested(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.offered["sk-proj-abcdef1234"] = []string{"gpt-6-luna", "gpt-6-sol"}

	saved, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider:     entity.AIProviderOpenAI,
		APIKey:       "  sk-proj-abcdef1234  ",
		DefaultModel: "gpt-6-luna",
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if h.store[workspaceID].apiKey != "sk-proj-abcdef1234" {
		t.Errorf("stored key = %q, want it trimmed of the whitespace a paste brings", h.store[workspaceID].apiKey)
	}

	if saved.KeyHint != "1234" || saved.Status() != entity.AIProviderUnverified {
		t.Errorf("saved = hint %q status %q, want hint 1234 and unverified", saved.KeyHint, saved.Status())
	}

	if !slices.Equal(h.actions(), []entity.AuditAction{entity.AuditAIProviderConfigured}) {
		t.Errorf("audited %v, want one configured entry", h.actions())
	}
}

func TestAModelTheKeyCannotUseIsRefused(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "gpt-6-luna")

	_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider:     entity.AIProviderOpenAI,
		DefaultModel: "gpt-5-turbo-legacy",
	})
	if !errors.Is(err, entity.ErrAIProviderModelUnavailable) {
		t.Fatalf("err = %v, want ErrAIProviderModelUnavailable", err)
	}

	if h.store[workspaceID].connection.DefaultModel != "gpt-6-luna" {
		t.Fatal("the refused model replaced the one that works")
	}
}

func TestChangingTheModelKeepsTheKeyAndAsksForANewTest(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "gpt-6-luna")
	h.offered["sk-live"] = []string{"gpt-6-luna", "gpt-6-sol"}
	h.probesAnswer(nil)

	if _, err := h.service.Test(context.Background(), workspaceID); err != nil {
		t.Fatalf("Test: %v", err)
	}

	saved, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider:     entity.AIProviderOpenAI,
		DefaultModel: "gpt-6-sol",
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if h.store[workspaceID].apiKey != "sk-live" {
		t.Error("changing only the model lost the stored key")
	}

	if saved.Status() != entity.AIProviderUnverified {
		t.Errorf("status = %q; a pass against the old model says nothing about the new one", saved.Status())
	}

	if !slices.Equal(h.actions(), []entity.AuditAction{entity.AuditAIProviderModelChanged}) {
		t.Errorf("audited %v, want one model_changed entry", h.actions())
	}
}

func TestATestRecordsWhatTheProviderSaid(t *testing.T) {
	cases := []struct {
		name    string
		answer  error
		status  entity.AIProviderStatus
		failure entity.AIProviderFailure
	}{
		{"a working key", nil, entity.AIProviderVerified, ""},
		{"a revoked key", fmt.Errorf("%w: Incorrect API key", entity.ErrAIProviderKeyRejected), entity.AIProviderFailed, entity.AIProviderFailureKeyRejected},
		{"a retired model", fmt.Errorf("%w: model_not_found", entity.ErrAIProviderModelUnavailable), entity.AIProviderFailed, entity.AIProviderFailureModelUnavailable},
		{"an empty account", entity.ErrAIProviderQuotaExceeded, entity.AIProviderFailed, entity.AIProviderFailureQuotaExceeded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			workspaceID := h.workspace(entity.MembershipRoleAdmin)
			h.configured(workspaceID, "sk-live", "gpt-6-luna")
			h.probesAnswer(tc.answer)

			_, err := h.service.Test(context.Background(), workspaceID)
			if !errors.Is(err, tc.answer) {
				t.Fatalf("err = %v, want %v handed back so the screen can say why", err, tc.answer)
			}

			recorded := h.store[workspaceID].connection
			if recorded.Status() != tc.status || recorded.Failure != tc.failure {
				t.Errorf("recorded status %q failure %q, want %q %q", recorded.Status(), recorded.Failure, tc.status, tc.failure)
			}
		})
	}
}

func TestATestWithoutADefaultModelAsksForOne(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "")

	_, err := h.service.Test(context.Background(), workspaceID)

	var validation entity.ValidationError
	if !errors.As(err, &validation) || validation.Fields[0].Field != "defaultModel" {
		t.Fatalf("err = %v, want defaultModel required", err)
	}
}

func TestEachWorkspaceIsTestedWithItsOwnKey(t *testing.T) {
	h := newHarness(t)
	northwind := h.workspace(entity.MembershipRoleAdmin)
	acme := h.workspace(entity.MembershipRoleAdmin)
	h.configured(northwind, "sk-northwind", "gpt-6-luna")
	h.configured(acme, "sk-acme", "gpt-6-luna")
	h.probesAnswer(nil)

	if _, err := h.service.Test(context.Background(), acme); err != nil {
		t.Fatalf("Test: %v", err)
	}

	if len(h.probed) != 1 || h.probed[0].apiKey != "sk-acme" {
		t.Fatalf("probed with %v, want only the key stored for the workspace under test", h.probed)
	}

	if h.store[northwind].connection.Status() != entity.AIProviderUnverified {
		t.Fatal("testing one workspace changed another workspace's status")
	}
}

func TestAnAdministratorOfOneWorkspaceCannotReachAnother(t *testing.T) {
	h := newHarness(t)
	h.workspace(entity.MembershipRoleAdmin)
	elsewhere := h.workspace(entity.MembershipRoleAdmin)
	delete(h.roles, elsewhere)
	h.configured(elsewhere, "sk-elsewhere", "gpt-6-luna")

	if _, err := h.service.Models(context.Background(), elsewhere); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("err = %v, want ErrAccountForbidden for a workspace the caller does not belong to", err)
	}
}

func TestRemovingTheKeyDeletesItAndIsAudited(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "gpt-6-luna")

	if err := h.service.Remove(context.Background(), workspaceID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, kept := h.store[workspaceID]; kept {
		t.Fatal("the key is still stored after removal")
	}

	if !slices.Equal(h.actions(), []entity.AuditAction{entity.AuditAIProviderRemoved}) {
		t.Errorf("audited %v, want one removed entry", h.actions())
	}

	if err := h.service.Remove(context.Background(), workspaceID); !errors.Is(err, entity.ErrAIProviderNotConfigured) {
		t.Errorf("second remove: err = %v, want ErrAIProviderNotConfigured", err)
	}
}

func TestMovingToAnotherEndpointNeedsTheKeyEnteredAgain(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "gpt-6-luna")

	_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider:     entity.AIProviderOpenAI,
		Endpoint:     entity.AIProviderEndpoint{BaseURL: "https://gateway.example.com/v1"},
		DefaultModel: "gpt-6-luna",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) || validation.Fields[0].Field != "apiKey" {
		t.Fatalf("err = %v, want apiKey required", err)
	}

	if len(h.listed) != 0 {
		t.Fatalf(
			"the stored key was sent to %v; a key somebody else pasted must never follow the "+
				"endpoint to a host the person changing it chose",
			h.listed,
		)
	}
}

func TestANewEndpointIsProvedWithItsKeyAndUsedForTests(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.configured(workspaceID, "sk-live", "gpt-6-luna")
	h.offered["sk-gateway"] = []string{"llama-4-scout"}
	h.probesAnswer(nil)

	gateway := entity.AIProviderEndpoint{BaseURL: "http://10.0.4.12:8000/v1", AllowPrivateAddress: true}

	saved, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider:     entity.AIProviderOpenAI,
		Endpoint:     entity.AIProviderEndpoint{BaseURL: gateway.BaseURL + "/", AllowPrivateAddress: true},
		APIKey:       "sk-gateway",
		DefaultModel: "llama-4-scout",
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if saved.Endpoint != gateway || !slices.Equal(h.listed, []entity.AIProviderEndpoint{gateway}) {
		t.Fatalf("saved %v after listing %v, want the new endpoint without its trailing slash", saved.Endpoint, h.listed)
	}

	if _, err := h.service.Test(context.Background(), workspaceID); err != nil {
		t.Fatalf("Test: %v", err)
	}

	if h.probed[0].endpoint != gateway {
		t.Errorf("tested against %v, want the workspace's endpoint", h.probed[0].endpoint)
	}

	want := []entity.AuditAction{
		entity.AuditAIProviderKeyReplaced, entity.AuditAIProviderEndpointChanged, entity.AuditAIProviderModelChanged,
	}
	if !slices.Equal(h.actions(), want) {
		t.Errorf("audited %v, want %v", h.actions(), want)
	}
}

func TestAMalformedEndpointIsRefusedBeforeAnythingIsSent(t *testing.T) {
	h := newHarness(t)
	workspaceID := h.workspace(entity.MembershipRoleAdmin)

	_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider: entity.AIProviderOpenAI,
		Endpoint: entity.AIProviderEndpoint{BaseURL: "ftp://models.example.com"},
		APIKey:   "sk-test",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) || validation.Fields[0].Field != "baseUrl" {
		t.Fatalf("err = %v, want baseUrl malformed", err)
	}

	if len(h.listed) != 0 {
		t.Fatal("a malformed endpoint was dialled")
	}
}

func TestACloudWorkspaceCannotReachAPrivateNetwork(t *testing.T) {
	h := newHarness(t)
	h.service = aiprovidersvc.New(h.connections, h.models, h.authorizer, h.audit, config.Instance{})
	workspaceID := h.workspace(entity.MembershipRoleAdmin)
	h.offered["sk-gateway"] = []string{"llama-4-scout"}

	_, err := h.service.Configure(context.Background(), workspaceID, entity.AIProviderInput{
		Provider: entity.AIProviderOpenAI,
		Endpoint: entity.AIProviderEndpoint{BaseURL: "http://10.0.4.12:8000/v1", AllowPrivateAddress: true},
		APIKey:   "sk-gateway",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) || validation.Fields[0].Field != "allowPrivateAddress" {
		t.Fatalf("err = %v, want allowPrivateAddress refused", err)
	}

	if len(h.listed) != 0 {
		t.Fatal(
			"a shared instance dialled a private address a tenant named, which would put its own " +
				"internal network one workspace setting away from every administrator",
		)
	}
}
