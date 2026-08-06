package issuecomment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	attachmentrepo "github.com/usenorn/norn/internal/repository/attachment"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuecommentrepo "github.com/usenorn/norn/internal/repository/issuecomment"
	issuefollowerrepo "github.com/usenorn/norn/internal/repository/issuefollower"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
)

type harness struct {
	comments    *issuecommentrepo.MockIssueComment
	attachments *attachmentrepo.MockAttachment
	issues      *issuerepo.MockIssue
	teams       *teamrepo.MockTeam
	activity    *activityrepo.MockActivity
	notify      *notificationeventrepo.MockNotificationEvent
	events      *eventsvc.MockEvents
	followers   *issuefollowerrepo.MockIssueFollower
	authorizer  *authorizersvc.MockAuthorizer
	service     service.IssueComments

	workspaceID uuid.UUID
	issueID     uuid.UUID
	teamID      uuid.UUID
	commentID   uuid.UUID
	authorID    uuid.UUID
	otherID     uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		comments:    issuecommentrepo.NewMockIssueComment(ctrl),
		attachments: attachmentrepo.NewMockAttachment(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		notify:      notificationeventrepo.NewMockNotificationEvent(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		followers:   issuefollowerrepo.NewMockIssueFollower(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		issueID:     uuid.New(),
		teamID:      uuid.New(),
		commentID:   uuid.New(),
		authorID:    uuid.New(),
		otherID:     uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = issuecommentsvc.New(
		h.comments, h.attachments, h.issues, h.teams, h.activity, h.notify, h.events,
		silentEmitter(ctrl), h.followers,
		agenthold.New(agentsettingrepo.NewMockAgentSetting(ctrl), agentproposalrepo.NewMockAgentProposal(ctrl), agentrepo.NewMockAgent(ctrl)),
		h.authorizer, transactor,
	)

	h.notify.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()
	h.followers.EXPECT().Follow(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return h
}

func (h *harness) actAs(accountID uuid.UUID, role entity.MembershipRole) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID},
			Role:  role,
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()
}

func (h *harness) seesTheIssue() {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{ID: h.issueID, WorkspaceID: h.workspaceID, TeamID: h.teamID}, nil).
		AnyTimes()
}

func (h *harness) cannotSeeTheIssue() {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{}, entity.ErrIssueNotFound).
		AnyTimes()
}

func (h *harness) holds(comment entity.IssueComment) {
	h.comments.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(comment, nil).
		AnyTimes()
}

func (h *harness) comment() entity.IssueComment {
	return entity.IssueComment{
		ID:              h.commentID,
		WorkspaceID:     h.workspaceID,
		IssueID:         h.issueID,
		AuthorAccountID: h.authorID,
		Body:            "the uploader retries three times",
	}
}

func (h *harness) accepts() {
	h.comments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, comment entity.IssueComment) (entity.IssueComment, error) {
			comment.ID = h.commentID

			return comment, nil
		}).
		AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.attachments.EXPECT().
		ClaimForComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
}

func TestCommentingOnAnIssueYouCannotSeeIsIndistinguishableFromItNotExisting(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.cannotSeeTheIssue()

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body: "is this the same bug?",
	})

	if !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf(
			"posting on an invisible issue returned %v. Commenting must never become a way to "+
				"probe for issues a private team is keeping to itself.",
			err,
		)
	}
}

func TestAViewerMayCommentEvenThoughTheyMayNotTouchTheIssue(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleViewer)
	h.seesTheIssue()
	h.accepts()
	h.comments.EXPECT().RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	posted, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body: "this reproduces on my machine too",
	})
	if err != nil {
		t.Fatalf(
			"a viewer could not comment: %v. The role is described to users as \"Reads and "+
				"comments\", so refusing here would make that copy a lie on screen.",
			err,
		)
	}

	if posted.Comment.Body == "" {
		t.Fatal("the posted comment came back empty")
	}
}

func TestAnImportedCommentReachesTheRepositoryWithTheDatesItsSourceRecorded(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	var captured entity.IssueComment

	h.comments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, comment entity.IssueComment) (entity.IssueComment, error) {
			captured = comment
			comment.ID = h.commentID

			return comment, nil
		})

	h.accepts()
	h.comments.EXPECT().RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(72 * time.Hour)
	author := uuid.New()
	origin := entity.NewImportOrigin(createdAt, updatedAt, author)

	if _, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:   "this reproduces on my machine too",
		Origin: &origin,
	}); err != nil {
		t.Fatalf("Post: %v", err)
	}

	if captured.Origin == nil {
		t.Fatal("the origin stopped at the service, so the comment would be dated the moment the import ran")
	}

	gotCreated, gotUpdated := captured.Origin.Stamp(time.Now().UTC())

	if !gotCreated.Equal(createdAt) || !gotUpdated.Equal(updatedAt) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", gotCreated, gotUpdated, createdAt, updatedAt)
	}

	if captured.AuthorAccountID != author {
		t.Fatalf(
			"the comment was authored by %v rather than the member its source author was mapped "+
				"onto. Dating a comment three years ago and then signing it with whoever ran the "+
				"import is worse than not importing it: the thread reads as though one person "+
				"held every side of the conversation.",
			captured.AuthorAccountID,
		)
	}
}

func TestACommentWithNoImportBehindItIsStillAuthoredByWhoeverPostedIt(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	var captured entity.IssueComment

	h.comments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, comment entity.IssueComment) (entity.IssueComment, error) {
			captured = comment
			comment.ID = h.commentID

			return comment, nil
		})

	h.accepts()
	h.comments.EXPECT().RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body: "still broken",
	}); err != nil {
		t.Fatalf("Post: %v", err)
	}

	if captured.AuthorAccountID != h.authorID {
		t.Errorf("author = %v, want the actor %v", captured.AuthorAccountID, h.authorID)
	}

	if captured.Origin != nil {
		t.Error("an ordinary comment arrived carrying an import origin")
	}
}

func TestAMentionOfSomeoneWhoCannotSeeTheIssueIsRecordedAndReportedButNotNotifiable(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()
	h.accepts()

	h.comments.EXPECT().
		Audience(gomock.Any(), h.workspaceID, h.teamID, gomock.Any()).
		Return([]repository.CommentAudience{
			{AccountID: h.otherID, Name: "Rae Whitfield", Visible: false},
		}, nil)

	var recorded []entity.CommentMention

	h.comments.EXPECT().
		RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, mentions []entity.CommentMention) error {
			recorded = mentions

			return nil
		})

	posted, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:     "any idea about this?",
		Mentions: []service.CommentMentionInput{{Kind: entity.MentionKindAccount, AccountID: h.otherID}},
	})
	if err != nil {
		t.Fatalf("mentioning someone outside the issue's audience refused the comment: %v", err)
	}

	if len(recorded) != 1 || recorded[0].Visible {
		t.Fatalf(
			"the mention was recorded as %+v. It must be stored unreachable: a notification "+
				"system reads that flag, and nothing about this issue may travel to someone who "+
				"cannot open it.",
			recorded,
		)
	}

	if recorded[0].Name != "Rae Whitfield" {
		t.Fatalf(
			"the mention stored %q as the name. The name is snapshotted so the mention stays "+
				"resolvable after the person is removed from the workspace.",
			recorded[0].Name,
		)
	}

	if len(posted.Unreachable) != 1 || posted.Unreachable[0].AccountID != h.otherID {
		t.Fatalf(
			"the author was told about %d unreachable mentions. They have to learn at the moment "+
				"it matters that this person was not reached, or they wait for a reply that will "+
				"never come.",
			len(posted.Unreachable),
		)
	}
}

func TestAMentionOfSomeoneInTheAudienceIsNotReportedAsMissed(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()
	h.accepts()

	h.comments.EXPECT().
		Audience(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]repository.CommentAudience{
			{AccountID: h.otherID, Name: "Rae Whitfield", Visible: true},
		}, nil)
	h.comments.EXPECT().RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	posted, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:     "any idea about this?",
		Mentions: []service.CommentMentionInput{{Kind: entity.MentionKindAccount, AccountID: h.otherID}},
	})
	if err != nil {
		t.Fatalf("posting refused: %v", err)
	}

	if len(posted.Unreachable) != 0 {
		t.Fatalf("someone who can read the issue was reported as unreachable: %+v", posted.Unreachable)
	}
}

func TestMentioningSomeoneOutsideTheWorkspaceIsRefusedRatherThanRecorded(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	h.comments.EXPECT().
		Audience(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil)

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:     "any idea about this?",
		Mentions: []service.CommentMentionInput{{Kind: entity.MentionKindAccount, AccountID: uuid.New()}},
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"mentioning a stranger returned %v. The members list the picker draws from is "+
				"workspace-wide, so an id from outside it is a client bug, not a person to "+
				"snapshot a name for.",
			err,
		)
	}
}

func TestMentioningATeamOutsideYourScopeIsRefused(t *testing.T) {
	h := newHarness(t)
	foreign := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.authorID},
			Role:  entity.MembershipRoleMember,
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, TeamIDs: []uuid.UUID{h.teamID}},
		}, nil).
		AnyTimes()
	h.seesTheIssue()

	h.comments.EXPECT().Audience(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), foreign).
		Return(entity.Team{ID: foreign, WorkspaceID: h.workspaceID, Name: "Security"}, nil)

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:     "whose call is this?",
		Mentions: []service.CommentMentionInput{{Kind: entity.MentionKindTeam, TeamID: foreign}},
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"mentioning a private team the author cannot see returned %v. The team read is "+
				"unscoped, so this service is the only thing standing between a picker and the "+
				"name of a team nobody said they could know about.",
			err,
		)
	}
}

func TestTooManyMentionsIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	mentions := make([]service.CommentMentionInput, entity.CommentMaxMentions+1)
	for i := range mentions {
		mentions[i] = service.CommentMentionInput{Kind: entity.MentionKindAccount, AccountID: uuid.New()}
	}

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		Body:     "everyone look at this",
		Mentions: mentions,
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"%d mentions on one comment returned %v. Nothing here rate-limits an authenticated "+
				"write, so this cap is the only bound on the fan-out of a single post.",
			len(mentions), err,
		)
	}
}

func TestAReplyMayOnlyHangOffATopLevelComment(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	reply := h.comment()
	reply.ParentCommentID = uuid.New()
	h.holds(reply)

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		ParentCommentID: h.commentID,
		Body:            "and another thing",
	})

	if !errors.Is(err, entity.ErrIssueCommentNotReplyable) {
		t.Fatalf(
			"replying to a reply returned %v. The database refuses it through the root-marker "+
				"foreign key, so without this the caller would see a raw constraint violation "+
				"as a 500.",
			err,
		)
	}
}

func TestAReplyCannotBeGraftedOntoACommentFromAnotherIssue(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	elsewhere := h.comment()
	elsewhere.IssueID = uuid.New()
	h.holds(elsewhere)

	_, err := h.service.Post(context.Background(), h.workspaceID, h.issueID, service.PostCommentInput{
		ParentCommentID: h.commentID,
		Body:            "related",
	})

	if !errors.Is(err, entity.ErrIssueCommentNotFound) {
		t.Fatalf("replying across issues returned %v", err)
	}
}

func TestAReplyToADeletedCommentIsStillAllowed(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()
	h.accepts()

	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	tombstone := h.comment()
	tombstone.Body = ""
	tombstone.DeletedAt = &at
	h.holds(tombstone)
	h.comments.EXPECT().RecordMentions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.Post(
		context.Background(), h.workspaceID, h.issueID,
		service.PostCommentInput{ParentCommentID: h.commentID, Body: "for the record, we shipped it"},
	); err != nil {
		t.Fatalf(
			"replying under a tombstone was refused: %v. A tombstone exists precisely so the "+
				"replies beneath it still hang off something.",
			err,
		)
	}
}

func TestOnlyTheAuthorMayEdit(t *testing.T) {
	for name, actor := range map[string]struct {
		accountID uuid.UUID
		role      entity.MembershipRole
		allowed   bool
	}{
		"the author":        {uuid.Nil, entity.MembershipRoleMember, true},
		"another member":    {uuid.New(), entity.MembershipRoleMember, false},
		"a workspace admin": {uuid.New(), entity.MembershipRoleAdmin, false},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			accountID := actor.accountID
			if accountID == uuid.Nil {
				accountID = h.authorID
			}

			h.actAs(accountID, actor.role)
			h.seesTheIssue()
			h.holds(h.comment())

			if actor.allowed {
				h.comments.EXPECT().
					Edit(gomock.Any(), h.commentID, "now with a stack trace", gomock.Any()).
					Return(nil)
			}

			_, err := h.service.Edit(
				context.Background(), h.workspaceID, h.issueID, h.commentID,
				service.EditCommentInput{Body: "now with a stack trace"},
			)

			if actor.allowed && err != nil {
				t.Fatalf("the author could not edit their own comment: %v", err)
			}

			if !actor.allowed && !errors.Is(err, entity.ErrIssueCommentNotAuthor) {
				t.Fatalf(
					"%s edited someone else's words and got %v. Editing rewrites what a person "+
						"said under their own name; an administrator may remove it but never "+
						"rephrase it.",
					name, err,
				)
			}
		})
	}
}

func TestDeletionAdmitsTheAuthorAndAnAdminAndNobodyElse(t *testing.T) {
	for name, actor := range map[string]struct {
		accountID uuid.UUID
		role      entity.MembershipRole
		allowed   bool
	}{
		"the author":        {uuid.Nil, entity.MembershipRoleMember, true},
		"a workspace admin": {uuid.New(), entity.MembershipRoleAdmin, true},
		"another member":    {uuid.New(), entity.MembershipRoleMember, false},
		"a viewer":          {uuid.New(), entity.MembershipRoleViewer, false},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			accountID := actor.accountID
			if accountID == uuid.Nil {
				accountID = h.authorID
			}

			h.actAs(accountID, actor.role)
			h.seesTheIssue()
			h.holds(h.comment())

			var recorded entity.Activity

			if actor.allowed {
				h.comments.EXPECT().Tombstone(gomock.Any(), h.commentID, gomock.Any()).Return(nil)
				h.activity.EXPECT().
					Record(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, activity entity.Activity) error {
						recorded = activity

						return nil
					})
			}

			err := h.service.Remove(context.Background(), h.workspaceID, h.issueID, h.commentID)

			if actor.allowed {
				if err != nil {
					t.Fatalf("%s could not delete the comment: %v", name, err)
				}

				if recorded.Kind != entity.ActivityKindCommentDeleted {
					t.Fatalf(
						"deleting recorded %q in the activity feed. A comment appearing and then "+
							"vanishing is a change to the issue that the feed has to show.",
						recorded.Kind,
					)
				}

				return
			}

			if !errors.Is(err, entity.ErrIssueCommentNotDeletable) {
				t.Fatalf("%s deleted a comment they do not own and got %v", name, err)
			}
		})
	}
}

func TestNeitherEditingNorDeletingLandsOnATombstoneTwice(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	t.Run("editing", func(t *testing.T) {
		h := newHarness(t)
		h.actAs(h.authorID, entity.MembershipRoleMember)
		h.seesTheIssue()

		tombstone := h.comment()
		tombstone.Body = ""
		tombstone.DeletedAt = &at
		h.holds(tombstone)

		_, err := h.service.Edit(
			context.Background(), h.workspaceID, h.issueID, h.commentID,
			service.EditCommentInput{Body: "actually, ignore me"},
		)

		if !errors.Is(err, entity.ErrIssueCommentDeleted) {
			t.Fatalf("a deleted comment was edited back into existence: %v", err)
		}
	})

	t.Run("deleting", func(t *testing.T) {
		h := newHarness(t)
		h.actAs(h.authorID, entity.MembershipRoleMember)
		h.seesTheIssue()

		tombstone := h.comment()
		tombstone.Body = ""
		tombstone.DeletedAt = &at
		h.holds(tombstone)

		if err := h.service.Remove(
			context.Background(), h.workspaceID, h.issueID, h.commentID,
		); !errors.Is(err, entity.ErrIssueCommentDeleted) {
			t.Fatalf(
				"deleting twice returned %v — the second delete would write a second activity "+
					"entry for something that happened once",
				err,
			)
		}
	})
}

func TestAnEmptyEditIsRefusedSoAnEditCannotImpersonateADeletion(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()
	h.holds(h.comment())

	_, err := h.service.Edit(
		context.Background(), h.workspaceID, h.issueID, h.commentID, service.EditCommentInput{Body: "   "},
	)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"an edit to whitespace returned %v. The table refuses a live comment with an empty "+
				"body, so this would surface a constraint violation as a 500 — and it would let "+
				"an edit erase words without recording the deletion.",
			err,
		)
	}
}

func TestReactingToADeletedCommentIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.otherID, entity.MembershipRoleMember)
	h.seesTheIssue()

	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	tombstone := h.comment()
	tombstone.Body = ""
	tombstone.DeletedAt = &at
	h.holds(tombstone)

	if _, err := h.service.React(
		context.Background(), h.workspaceID, h.issueID, h.commentID, entity.CommentReactionUp,
	); !errors.Is(err, entity.ErrIssueCommentDeleted) {
		t.Fatalf("a tombstone collected a reaction: %v", err)
	}
}

func TestAnUnnamedReactionIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.otherID, entity.MembershipRoleMember)
	h.seesTheIssue()
	h.holds(h.comment())

	_, err := h.service.React(context.Background(), h.workspaceID, h.issueID, h.commentID, "rocket")

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"an unnamed reaction returned %v. The column has a CHECK, so anything the product "+
				"does not name would fail at the database instead of here.",
			err,
		)
	}
}

func TestAFullPageHandsBackACursorPointingAtItsLastRoot(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	const limit = 3

	roots := make([]entity.IssueComment, 0, limit+1)
	for i := range limit + 1 {
		roots = append(roots, entity.IssueComment{
			ID:        uuid.New(),
			IssueID:   h.issueID,
			Body:      "one of many",
			CreatedAt: time.Date(2026, 8, 4, 9, i, 0, 0, time.UTC),
		})
	}

	h.comments.EXPECT().
		ListThread(gomock.Any(), h.issueID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.CommentPage) ([]entity.IssueComment, error) {
			if page.Limit != limit+1 {
				t.Fatalf(
					"the thread was read with a limit of %d. One row beyond the page is how the "+
						"service knows there is more without counting anything.",
					page.Limit,
				)
			}

			return roots, nil
		})

	thread, err := h.service.List(
		context.Background(), h.workspaceID, h.issueID, service.ListCommentsInput{Limit: limit},
	)
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}

	if len(thread.Comments) != limit {
		t.Fatalf("the page returned %d comments, want %d", len(thread.Comments), limit)
	}

	if thread.NextCursor == "" {
		t.Fatal("a full page handed back no cursor, so the rest of the conversation is unreachable")
	}

	cursor, err := entity.DecodeCommentCursor(thread.NextCursor)
	if err != nil {
		t.Fatalf("the cursor we handed out will not decode: %v", err)
	}

	if cursor.CommentID != roots[limit-1].ID {
		t.Fatal(
			"the cursor does not point at the last comment on the page, so the next page would " +
				"skip a comment or repeat one",
		)
	}
}

func TestAShortPageEndsTheThread(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.authorID, entity.MembershipRoleMember)
	h.seesTheIssue()

	h.comments.EXPECT().
		ListThread(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.IssueComment{{ID: uuid.New(), Body: "only one"}}, nil)

	thread, err := h.service.List(
		context.Background(), h.workspaceID, h.issueID, service.ListCommentsInput{Limit: 25},
	)
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}

	if thread.NextCursor != "" {
		t.Fatal("a partial page handed back a cursor, so the reader is invited to load nothing")
	}
}

func TestReadingCommentsOnAnInvisibleIssueIsRefusedBeforeTheThreadIsTouched(t *testing.T) {
	h := newHarness(t)
	h.actAs(h.otherID, entity.MembershipRoleMember)
	h.cannotSeeTheIssue()

	if _, err := h.service.List(
		context.Background(), h.workspaceID, h.issueID, service.ListCommentsInput{},
	); !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("reading a private team's conversation returned %v", err)
	}
}
