package label_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	labelgrouprepo "github.com/usenorn/norn/internal/repository/labelgroup"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	labelsvc "github.com/usenorn/norn/internal/service/label"
)

type harness struct {
	labels     *labelrepo.MockLabel
	groups     *labelgrouprepo.MockLabelGroup
	teams      *teamrepo.MockTeam
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.Labels
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		labels:     labelrepo.NewMockLabel(ctrl),
		groups:     labelgrouprepo.NewMockLabelGroup(ctrl),
		teams:      teamrepo.NewMockTeam(ctrl),
		authorizer: authorizersvc.NewMockAuthorizer(ctrl),
		transactor: transactorrepo.NewMockTransactor(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = labelsvc.New(h.labels, h.groups, h.teams, h.authorizer, h.transactor)

	return h
}

func (h *harness) actorSeesEveryTeam(workspaceID uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Role:  entity.MembershipRoleAdmin,
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil)
}

func (h *harness) actorSeesOnly(workspaceID uuid.UUID, teamIDs ...uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Role:  entity.MembershipRoleMember,
			Scope: entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: teamIDs},
		}, nil)
}

func (h *harness) expectTeam(workspaceID, teamID uuid.UUID, status entity.TeamStatus) {
	h.teams.EXPECT().
		GetByID(gomock.Any(), teamID).
		Return(entity.Team{
			ID:          teamID,
			WorkspaceID: workspaceID,
			Key:         "MOB",
			Name:        "Mobile",
			Status:      status,
		}, nil).
		AnyTimes()
}

func workspaceLabel(workspaceID uuid.UUID, name string) entity.Label {
	return entity.Label{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       entity.LabelColorCyan,
	}
}

func teamLabel(workspaceID, teamID uuid.UUID, name string) entity.Label {
	label := workspaceLabel(workspaceID, name)
	label.TeamID = teamID

	return label
}

func grouped(label entity.Label, groupID uuid.UUID) entity.Label {
	label.GroupID = groupID

	return label
}
