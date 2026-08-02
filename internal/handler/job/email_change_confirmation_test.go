package job_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/job"
	accountsvc "github.com/usenorn/norn/internal/service/account"
)

func newTask(t *testing.T, payload entity.EmailChangeConfirmationPayload) *asynq.Task {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return asynq.NewTask(entity.TaskTypeEmailChangeConfirmation, encoded)
}

func TestAMalformedPayloadIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewEmailChangeConfirmationHandler(accounts)

	task := asynq.NewTask(entity.TaskTypeEmailChangeConfirmation, []byte("not json"))

	err := handler.ProcessTask(context.Background(), task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}

func TestAnIncompletePayloadIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewEmailChangeConfirmationHandler(accounts)

	err := handler.ProcessTask(context.Background(), newTask(t, entity.EmailChangeConfirmationPayload{
		EmailChangeID: uuid.New(),
	}))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}

func TestAValidPayloadDelegatesTheDeliveryToTheAccountsService(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewEmailChangeConfirmationHandler(accounts)

	changeID := uuid.New()

	accounts.EXPECT().SendEmailChangeConfirmation(gomock.Any(), changeID, "a-token").Return(nil)

	if err := handler.ProcessTask(context.Background(), newTask(t, entity.EmailChangeConfirmationPayload{
		EmailChangeID: changeID,
		Token:         "a-token",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
}

func TestATransientDeliveryFailureIsLeftRetryable(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewEmailChangeConfirmationHandler(accounts)

	changeID := uuid.New()
	transient := errors.New("smtp server unreachable")

	accounts.EXPECT().SendEmailChangeConfirmation(gomock.Any(), changeID, "a-token").Return(transient)

	err := handler.ProcessTask(context.Background(), newTask(t, entity.EmailChangeConfirmationPayload{
		EmailChangeID: changeID,
		Token:         "a-token",
	}))

	if !errors.Is(err, transient) {
		t.Fatalf("ProcessTask error = %v, want the transient failure", err)
	}

	if errors.Is(err, asynq.SkipRetry) {
		t.Fatal("a transient delivery failure must stay retryable")
	}
}
