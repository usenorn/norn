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
	knowledgeindexsvc "github.com/usenorn/norn/internal/service/knowledgeindex"
)

func newKnowledgeRefreshTask(t *testing.T, payload entity.KnowledgeRefreshPayload) *asynq.Task {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return asynq.NewTask(entity.TaskTypeKnowledgeRefresh, encoded)
}

func TestAKnowledgeRefreshWithoutAWorkspaceIsNotRetried(t *testing.T) {
	handler := job.NewKnowledgeRefreshHandler(knowledgeindexsvc.NewMockKnowledgeIndexes(gomock.NewController(t)))

	for name, task := range map[string]*asynq.Task{
		"not json":     asynq.NewTask(entity.TaskTypeKnowledgeRefresh, []byte("not json")),
		"no workspace": newKnowledgeRefreshTask(t, entity.KnowledgeRefreshPayload{}),
	} {
		if err := handler.ProcessTask(context.Background(), task); !errors.Is(err, asynq.SkipRetry) {
			t.Errorf("%s: err = %v, want asynq.SkipRetry", name, err)
		}
	}
}

func TestAProviderFailureIsLeftToTheSweepWhileAStoreFailureIsRetried(t *testing.T) {
	for name, testCase := range map[string]struct {
		err      error
		retried  bool
		reported error
	}{
		"a rejected key":    {err: entity.ErrAIProviderKeyRejected, reported: entity.ErrAIProviderKeyRejected},
		"a provider outage": {err: entity.ErrAIProviderUnreachable, reported: entity.ErrAIProviderUnreachable},
		"a database outage": {err: errors.New("connection reset"), retried: true},
	} {
		t.Run(name, func(t *testing.T) {
			indexes := knowledgeindexsvc.NewMockKnowledgeIndexes(gomock.NewController(t))
			workspaceID := uuid.New()

			indexes.EXPECT().Refresh(gomock.Any(), workspaceID).Return(testCase.err)

			err := job.NewKnowledgeRefreshHandler(indexes).ProcessTask(
				context.Background(), newKnowledgeRefreshTask(t, entity.KnowledgeRefreshPayload{WorkspaceID: workspaceID}),
			)

			if errors.Is(err, asynq.SkipRetry) == testCase.retried {
				t.Fatalf("err = %v, retried = %v, want retried = %v", err, !errors.Is(err, asynq.SkipRetry), testCase.retried)
			}

			if testCase.reported != nil && !errors.Is(err, testCase.reported) {
				t.Fatalf("err = %v, want it to still name %v", err, testCase.reported)
			}
		})
	}
}
