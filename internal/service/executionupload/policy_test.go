package executionupload_test

import (
	"context"
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAWorkspaceWithNoPolicyOfItsOwnKeepsWhatTheInstanceSays(t *testing.T) {
	h := newHarness(t)
	h.expectPolicyDecision()

	policy, err := h.service.Policy(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("read the execution policy: %v", err)
	}

	if policy.Telemetry != entity.TelemetryFull || policy.UploadRetentionDays != 90 {
		t.Fatalf("a workspace that has never been configured came back as %+v", policy)
	}
}

func TestARetentionWindowNobodyCouldMeanIsRefused(t *testing.T) {
	h := newHarness(t)
	h.expectPolicyDecision()

	_, err := h.service.SetPolicy(context.Background(), entity.WorkspaceExecutionPolicy{
		WorkspaceID: h.workspaceID, Telemetry: entity.TelemetryFull, UploadRetentionDays: 0,
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a retention window of zero days answered %v", err)
	}
}

func TestAMachineReadsTheTelemetryModeOfItsOwnWorkspaceWithoutHoldingTheWorkspace(t *testing.T) {
	h := newHarness(t)

	h.policy = entity.WorkspaceExecutionPolicy{
		WorkspaceID: h.workspaceID, Telemetry: entity.TelemetryMinimal, UploadRetentionDays: 30,
	}

	mode, err := h.service.Telemetry(context.Background())
	if err != nil {
		t.Fatalf("read the telemetry mode: %v", err)
	}

	if mode != entity.TelemetryMinimal {
		t.Fatalf("the machine was told telemetry is %q", mode)
	}
}
