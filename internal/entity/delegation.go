package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const IssueBriefMaxLen = 4000

var (
	ErrIssueDelegationNotFound      = errors.New("issue delegation not found")
	ErrIssueDelegationHeld          = errors.New("this issue is already delegated")
	ErrIssueDelegationAgentUnusable = errors.New("that agent cannot take work")
	ErrIssueDelegationUnassigned    = errors.New("an issue nobody is assigned cannot be delegated")
)

type PermissionProfile string

const (
	ProfileStrict       PermissionProfile = channelv1.ProfileStrict
	ProfileStandard     PermissionProfile = channelv1.ProfileStandard
	ProfileUnrestricted PermissionProfile = channelv1.ProfileUnrestricted
)

func PermissionProfiles() []PermissionProfile {
	return []PermissionProfile{ProfileStrict, ProfileStandard, ProfileUnrestricted}
}

func (p PermissionProfile) Valid() bool {
	return slices.Contains(PermissionProfiles(), p)
}

type BaseRefPolicy string

const (
	BaseRefOriginDefault BaseRefPolicy = channelv1.BaseRefOriginDefault
	BaseRefHead          BaseRefPolicy = channelv1.BaseRefHead
)

func BaseRefPolicies() []BaseRefPolicy {
	return []BaseRefPolicy{BaseRefOriginDefault, BaseRefHead}
}

func (p BaseRefPolicy) Valid() bool {
	return slices.Contains(BaseRefPolicies(), p)
}

type RuntimeChoice string

const (
	RuntimeChoiceAuto    RuntimeChoice = "auto"
	RuntimeChoiceProcess RuntimeChoice = RuntimeChoice(CodebaseRuntimeProcess)
	RuntimeChoiceDocker  RuntimeChoice = RuntimeChoice(CodebaseRuntimeDocker)
)

func RuntimeChoices() []RuntimeChoice {
	return []RuntimeChoice{RuntimeChoiceAuto, RuntimeChoiceProcess, RuntimeChoiceDocker}
}

func (c RuntimeChoice) Valid() bool {
	return slices.Contains(RuntimeChoices(), c)
}

func (c RuntimeChoice) Override() CodebaseRuntime {
	if c == RuntimeChoiceAuto {
		return ""
	}

	return CodebaseRuntime(c)
}

type DelegationParams struct {
	Tool         string
	Model        string
	Runtime      RuntimeChoice
	BaseRef      BaseRefPolicy
	IncludeDirty bool
	Profile      PermissionProfile
}

type IssueDelegation struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	IssueID              uuid.UUID
	AgentID              uuid.UUID
	AgentName            string
	AgentAccountID       uuid.UUID
	Brief                string
	Params               DelegationParams
	DelegatedByAccountID uuid.UUID
	DelegatedAt          time.Time
	RecalledByAccountID  uuid.UUID
	RecalledAt           *time.Time
}

func (d IssueDelegation) Open() bool {
	return d.RecalledAt == nil
}

func (p DelegationParams) Asked() ExecutionParams {
	return ExecutionParams{
		Tool:         p.Tool,
		Model:        p.Model,
		Runtime:      p.Runtime.Override(),
		BaseRef:      p.BaseRef,
		IncludeDirty: p.IncludeDirty,
		Profile:      p.Profile,
	}
}

func ValidateIssueBrief(field, brief string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(brief)) > IssueBriefMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateDelegationParams(field string, params DelegationParams) []FieldError {
	fields := []FieldError{
		delegationOptionalText(field+".tool", params.Tool, ExecutionToolMaxLen),
		delegationOptionalText(field+".model", params.Model, ExecutionModelMaxLen),
	}

	if params.Runtime != "" && !params.Runtime.Valid() {
		fields = append(fields, FieldError{
			Field: field + ".runtime",
			Code:  ValidationCodeUnsupportedValue,
		})
	}

	if params.BaseRef != "" && !params.BaseRef.Valid() {
		fields = append(fields, FieldError{
			Field: field + ".baseRef",
			Code:  ValidationCodeUnsupportedValue,
		})
	}

	if params.Profile != "" && !params.Profile.Valid() {
		fields = append(fields, FieldError{
			Field: field + ".permissionProfile",
			Code:  ValidationCodeUnsupportedValue,
		})
	}

	return fields
}

func delegationOptionalText(field, value string, limit int) FieldError {
	if strings.ContainsRune(value, 0) {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	if utf8.RuneCountInString(value) > limit {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}
