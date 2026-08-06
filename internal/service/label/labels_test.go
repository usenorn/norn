package label_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestALabelOnATeamOutsideTheActorsScopeIsHiddenRatherThanForbidden(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine := uuid.New()
	theirs := uuid.New()
	hidden := teamLabel(workspaceID, theirs, "Crash")

	h.actorSeesOnly(workspaceID, mine)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, hidden.ID).Return(hidden, nil)

	_, err := h.service.Usage(context.Background(), workspaceID, hidden.ID)

	if !errors.Is(err, entity.ErrLabelNotFound) {
		t.Fatalf("Usage error = %v, want ErrLabelNotFound", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal("a hidden label must not be distinguishable from a forbidden one")
	}
}

func TestCreatingATeamLabelForATeamOutsideTheScopeIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine := uuid.New()
	theirs := uuid.New()

	h.actorSeesOnly(workspaceID, mine)
	h.expectTeam(workspaceID, theirs, entity.TeamStatusActive)

	_, err := h.service.Create(context.Background(), service.CreateLabelInput{
		WorkspaceID: workspaceID,
		TeamID:      theirs,
		Name:        "Crash",
		Color:       entity.LabelColorCyan,
	})

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("Create error = %v, want ErrTeamNotFound", err)
	}
}

func TestCreatingALabelOnAnArchivedTeamIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.actorSeesEveryTeam(workspaceID)
	h.expectTeam(workspaceID, teamID, entity.TeamStatusArchived)

	_, err := h.service.Create(context.Background(), service.CreateLabelInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Name:        "Crash",
		Color:       entity.LabelColorCyan,
	})

	if !errors.Is(err, entity.ErrTeamArchived) {
		t.Fatalf("Create error = %v, want ErrTeamArchived", err)
	}
}

func TestAnImportedLabelReachesTheRepositoryWithTheDatesItsSourceRecorded(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.actorSeesEveryTeam(workspaceID)

	var captured entity.Label

	h.labels.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, label entity.Label) (entity.Label, error) {
			captured = label

			return label, nil
		})

	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(72 * time.Hour)
	origin := entity.NewImportOrigin(createdAt, updatedAt, uuid.New())

	if _, err := h.service.Create(context.Background(), service.CreateLabelInput{
		WorkspaceID: workspaceID,
		Name:        "Crash",
		Color:       entity.LabelColorCyan,
		Origin:      &origin,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Origin == nil {
		t.Fatal("the origin stopped at the service, so the label would be dated the moment the import ran")
	}

	gotCreated, gotUpdated := captured.Origin.Stamp(time.Now().UTC())

	if !gotCreated.Equal(createdAt) || !gotUpdated.Equal(updatedAt) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", gotCreated, gotUpdated, createdAt, updatedAt)
	}
}

func TestAnUnofferedColourIsRefusedBeforeItReachesTheStore(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.actorSeesEveryTeam(workspaceID)

	_, err := h.service.Create(context.Background(), service.CreateLabelInput{
		WorkspaceID: workspaceID,
		Name:        "Broken",
		Color:       entity.LabelColor("red"),
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "color" {
		t.Fatalf("validation names %q, want color", validation.Fields[0].Field)
	}
}

func TestRenamingALabelTouchesNothingButTheLabelRow(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	label := workspaceLabel(workspaceID, "bug")
	renamed := "Bug"

	h.actorSeesEveryTeam(workspaceID)
	h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)

	var (
		gotName  string
		gotColor entity.LabelColor
		gotGroup uuid.UUID
	)

	h.labels.EXPECT().
		UpdateSettings(gomock.Any(), label.ID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			id uuid.UUID,
			name string,
			color entity.LabelColor,
			groupID uuid.UUID,
		) (entity.Label, error) {
			gotName, gotColor, gotGroup = name, color, groupID
			updated := label
			updated.Name = name

			return updated, nil
		})

	h.labels.EXPECT().SyncApplicationGroup(gomock.Any(), label.ID, uuid.Nil).Return(nil)

	updated, err := h.service.Update(context.Background(), workspaceID, label.ID, service.UpdateLabelInput{
		Name: &renamed,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.ID != label.ID {
		t.Fatal("a rename must not mint a new id — every application and saved reference keys on it")
	}

	if gotName != renamed {
		t.Fatalf("stored name = %q, want %q", gotName, renamed)
	}

	if gotColor != label.Color || gotGroup != label.GroupID {
		t.Fatal("a rename must leave colour and group exactly as they were")
	}
}

func TestMergingMovesEveryApplicationAndRemovesTheSource(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	source := workspaceLabel(workspaceID, "bug")
	target := workspaceLabel(workspaceID, "Bug")

	h.actorSeesEveryTeam(workspaceID)
	h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, gomock.Any()).Return(source, nil).Times(2)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, source.ID).Return(source, nil)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, target.ID).Return(target, nil)

	var moved [2]entity.Label

	h.labels.EXPECT().
		MoveApplications(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, from, to entity.Label) error {
			moved = [2]entity.Label{from, to}

			return nil
		})

	h.labels.EXPECT().Delete(gomock.Any(), source.ID).Return(nil)

	merged, err := h.service.Merge(context.Background(), workspaceID, source.ID, target.ID)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if moved[0].ID != source.ID || moved[1].ID != target.ID {
		t.Fatalf("moved %v -> %v, want %v -> %v", moved[0].ID, moved[1].ID, source.ID, target.ID)
	}

	if merged.ID != target.ID {
		t.Fatalf("merged into %v, want %v", merged.ID, target.ID)
	}
}

func TestAMergeThatWouldNarrowScopeStrandsNothingBecauseItIsRefused(t *testing.T) {
	workspaceID := uuid.New()
	mobile := uuid.New()
	platform := uuid.New()

	cases := map[string]struct {
		source func() entity.Label
		target func() entity.Label
	}{
		"workspace into team": {
			source: func() entity.Label { return workspaceLabel(workspaceID, "Bug") },
			target: func() entity.Label { return teamLabel(workspaceID, mobile, "Crash") },
		},
		"across two teams": {
			source: func() entity.Label { return teamLabel(workspaceID, platform, "Bug") },
			target: func() entity.Label { return teamLabel(workspaceID, mobile, "Crash") },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			source, target := tc.source(), tc.target()

			h.actorSeesEveryTeam(workspaceID)
			h.expectTeam(workspaceID, mobile, entity.TeamStatusActive)
			h.expectTeam(workspaceID, platform, entity.TeamStatusActive)
			h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, gomock.Any()).Return(source, nil).Times(2)
			h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, source.ID).Return(source, nil)
			h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, target.ID).Return(target, nil)

			_, err := h.service.Merge(context.Background(), workspaceID, source.ID, target.ID)

			if !errors.Is(err, entity.ErrLabelMergeScopeNarrows) {
				t.Fatalf("Merge error = %v, want ErrLabelMergeScopeNarrows", err)
			}
		})
	}
}

func TestAMergeAcrossGroupsIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	severity := uuid.New()
	source := grouped(workspaceLabel(workspaceID, "blocker"), severity)
	target := workspaceLabel(workspaceID, "Blocker")

	h.actorSeesEveryTeam(workspaceID)
	h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, gomock.Any()).Return(source, nil).Times(2)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, source.ID).Return(source, nil)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, target.ID).Return(target, nil)

	_, err := h.service.Merge(context.Background(), workspaceID, source.ID, target.ID)

	if !errors.Is(err, entity.ErrLabelMergeGroupMismatch) {
		t.Fatalf("Merge error = %v, want ErrLabelMergeGroupMismatch", err)
	}
}

func TestRemovingALabelRefusesWhenItIsOnMoreIssuesThanWereAcknowledged(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	label := workspaceLabel(workspaceID, "Bug")

	h.actorSeesEveryTeam(workspaceID)
	h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)
	h.labels.EXPECT().Usage(gomock.Any(), label.ID, gomock.Any()).Return(entity.LabelUsage{Issues: 14}, nil)

	err := h.service.Remove(context.Background(), workspaceID, label.ID, 12)

	if !errors.Is(err, entity.ErrLabelUsageChanged) {
		t.Fatalf("Remove error = %v, want ErrLabelUsageChanged", err)
	}
}

func TestRemovingALabelAcceptsAnAcknowledgementThatCoversTheCurrentUsage(t *testing.T) {
	cases := map[string]struct {
		usage        int
		acknowledged int
	}{
		"exactly what was acknowledged": {12, 12},
		"fewer than acknowledged":       {9, 12},
		"unused label":                  {0, 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspaceID := uuid.New()
			label := workspaceLabel(workspaceID, "Bug")

			h.actorSeesEveryTeam(workspaceID)
			h.labels.EXPECT().LockByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)
			h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)
			h.labels.EXPECT().
				Usage(gomock.Any(), label.ID, gomock.Any()).
				Return(entity.LabelUsage{Issues: tc.usage}, nil)
			h.labels.EXPECT().Delete(gomock.Any(), label.ID).Return(nil)

			if err := h.service.Remove(context.Background(), workspaceID, label.ID, tc.acknowledged); err != nil {
				t.Fatalf("Remove: %v", err)
			}
		})
	}
}

func TestUsageIsCountedThroughTheActorsOwnScope(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine := uuid.New()
	label := workspaceLabel(workspaceID, "Bug")

	h.actorSeesOnly(workspaceID, mine)
	h.labels.EXPECT().GetByID(gomock.Any(), workspaceID, label.ID).Return(label, nil)

	var got entity.TeamScope

	h.labels.EXPECT().
		Usage(gomock.Any(), label.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, scope entity.TeamScope) (entity.LabelUsage, error) {
			got = scope

			return entity.LabelUsage{Issues: 3}, nil
		})

	if _, err := h.service.Usage(context.Background(), workspaceID, label.ID); err != nil {
		t.Fatalf("Usage: %v", err)
	}

	if got.AllTeams {
		t.Fatal("a scoped actor's usage must not be counted across every team")
	}

	if len(got.TeamIDs) != 1 || got.TeamIDs[0] != mine {
		t.Fatalf("usage counted over %v, want only %v", got.TeamIDs, mine)
	}
}

func TestRemovingAGroupUngroupsItsLabelsRatherThanDeletingThem(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	groupID := uuid.New()

	h.actorSeesEveryTeam(workspaceID)
	h.groups.EXPECT().
		GetByID(gomock.Any(), workspaceID, groupID).
		Return(entity.LabelGroup{ID: groupID, WorkspaceID: workspaceID, Name: "Severity"}, nil)

	ungrouped := false

	h.groups.EXPECT().
		Ungroup(gomock.Any(), groupID).
		DoAndReturn(func(context.Context, uuid.UUID) error {
			ungrouped = true

			return nil
		})

	h.groups.EXPECT().
		Delete(gomock.Any(), groupID).
		DoAndReturn(func(context.Context, uuid.UUID) error {
			if !ungrouped {
				t.Error("the group was deleted before its labels were released")
			}

			return nil
		})

	if err := h.service.RemoveGroup(context.Background(), workspaceID, groupID); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}
}

func TestTheOnlyLabelTallyIsTheDeletionAcknowledgement(t *testing.T) {
	usage := reflect.TypeOf(entity.LabelUsage{})

	if usage.NumField() != 1 || usage.Field(0).Type.Kind() != reflect.Int {
		t.Fatalf(
			"entity.LabelUsage has %d fields, want exactly one plain int; "+
				"a per-team or per-state breakdown would leak what the team scope conceals",
			usage.NumField(),
		)
	}

	carriesInt := func(t reflect.Type) bool {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}

		if t.Kind() == reflect.Int {
			return true
		}

		if t.Kind() != reflect.Struct {
			return false
		}

		for i := range t.NumField() {
			if t.Field(i).Type.Kind() == reflect.Int {
				return true
			}
		}

		return false
	}

	surfaces := map[string]reflect.Type{
		"repository.Label": reflect.TypeOf((*repository.Label)(nil)).Elem(),
		"service.Labels":   reflect.TypeOf((*service.Labels)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		tallies := []string{}

		for i := range surface.NumMethod() {
			method := surface.Method(i)

			for out := range method.Type.NumOut() {
				if !carriesInt(method.Type.Out(out)) {
					continue
				}

				if method.Type.Out(out) != usage {
					t.Errorf(
						"%s.%s returns %s, which puts a count on the wire outside the deletion acknowledgement",
						name, method.Name, method.Type.Out(out),
					)

					continue
				}

				tallies = append(tallies, method.Name)
			}
		}

		if len(tallies) != 1 {
			t.Errorf("%s exposes %v as tallies, want exactly one: the deletion-usage lookup", name, tallies)
		}
	}

	for _, model := range []reflect.Type{reflect.TypeOf(entity.Label{}), reflect.TypeOf(entity.LabelGroup{})} {
		for i := range model.NumField() {
			if model.Field(i).Type.Kind() == reflect.Int {
				t.Errorf("%s carries an int field %q, which would travel to every reader", model, model.Field(i).Name)
			}
		}
	}
}
