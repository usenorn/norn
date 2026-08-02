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

func newResetTask(t *testing.T, payload entity.PasswordResetPayload) *asynq.Task {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return asynq.NewTask(entity.TaskTypePasswordReset, encoded)
}

func TestAMalformedPasswordResetPayloadIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewPasswordResetHandler(accounts)

	task := asynq.NewTask(entity.TaskTypePasswordReset, []byte("not json"))

	err := handler.ProcessTask(context.Background(), task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}

func TestAPasswordResetPayloadWithoutATokenIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewPasswordResetHandler(accounts)

	err := handler.ProcessTask(context.Background(), newResetTask(t, entity.PasswordResetPayload{
		PasswordResetID: uuid.New(),
	}))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}

func TestAValidPasswordResetPayloadDelegatesTheDeliveryToTheAccountsService(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewPasswordResetHandler(accounts)

	resetID := uuid.New()

	accounts.EXPECT().SendPasswordReset(gomock.Any(), resetID, "a-token").Return(nil)

	if err := handler.ProcessTask(context.Background(), newResetTask(t, entity.PasswordResetPayload{
		PasswordResetID: resetID,
		Token:           "a-token",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
}

func TestAMalformedSSONoticePayloadIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewPasswordResetSSONoticeHandler(accounts)

	task := asynq.NewTask(entity.TaskTypePasswordResetSSONotice, []byte("not json"))

	err := handler.ProcessTask(context.Background(), task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}

func TestAnSSONoticePayloadWithoutAnAccountIsNotRetried(t *testing.T) {
	accounts := accountsvc.NewMockAccounts(gomock.NewController(t))
	handler := job.NewPasswordResetSSONoticeHandler(accounts)

	encoded, err := json.Marshal(entity.PasswordResetSSONoticePayload{})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	task := asynq.NewTask(entity.TaskTypePasswordResetSSONotice, encoded)

	if err := handler.ProcessTask(context.Background(), task); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask error = %v, want asynq.SkipRetry", err)
	}
}
