package entity

const (
	KnowledgeEmbedBatchSize      = 64
	KnowledgeStaleBatchSize      = 256
	KnowledgePassageBatchSize    = 100
	KnowledgeSweepWorkspaceLimit = 500
)

type KnowledgeText struct {
	ContentHash []byte
	Text        string
}

type KnowledgeEmbedding struct {
	ContentHash []byte
	Vector      []float32
}

func KnowledgeEmbeddedStatuses() []MemoryStatus {
	return []MemoryStatus{MemoryStatusProposed, MemoryStatusAccepted, MemoryStatusDisputed}
}
