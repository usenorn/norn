package service

import (
	"time"

	"github.com/usenorn/norn/internal/entity"
)

type NewCheckInput struct {
	Statement string
	Method    entity.CheckMethod
	Proof     string
	TimeLimit *time.Duration
}

type AddChecksInput struct {
	Checks []NewCheckInput
}

type DecideCheckInput struct {
	Approval entity.CheckApproval
}

type WaiveCheckInput struct {
	Reason string
}

type DeclareGapInput struct {
	Reason string
	Title  string
}

type SubmitEvidenceInput struct {
	Verdict    entity.EvidenceVerdict
	Channel    entity.EvidenceChannel
	Command    string
	Output     string
	ExitCode   *int
	ObservedAt time.Time
}
