package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const OverrideAcknowledged = "acknowledged"

const (
	CheckStatementMinLen = 3
	CheckStatementMaxLen = 300
	CheckProofMaxLen     = 2000
	CheckReasonMaxLen    = 2000
	ChecksPerIssueMax    = 40

	CheckTimeLimitMin = time.Hour
	CheckTimeLimitMax = 365 * 24 * time.Hour
)

var (
	ErrCheckNotFound            = errors.New("check not found")
	ErrCheckLimitReached        = errors.New("this issue already holds as many checks as it may")
	ErrCheckSettled             = errors.New("check has already been waived or declared a gap")
	ErrCheckDecided             = errors.New("check has already been approved or declined")
	ErrCheckDeclined            = errors.New("check was declined and no longer takes evidence")
	ErrCheckNotApproved         = errors.New("check has not been approved yet")
	ErrCheckDecisionNotPersonal = errors.New("only a person can decide a check")
	ErrCheckWaiverNotPersonal   = errors.New("only a person can waive a check")

	ErrIssueChecksUnproven = errors.New("issue has approved checks that are not proven")
)

type IssueChecksUnprovenError struct {
	Checks []Check
}

func (e IssueChecksUnprovenError) Error() string {
	return ErrIssueChecksUnproven.Error() + ": " + CheckStatements(e.Checks)
}

func (e IssueChecksUnprovenError) Unwrap() error {
	return ErrIssueChecksUnproven
}

type CheckMethod string

const (
	CheckMethodCommand     CheckMethod = "command"
	CheckMethodObservation CheckMethod = "observation"
	CheckMethodManual      CheckMethod = "manual"
	CheckMethodRegression  CheckMethod = "regression"
)

func CheckMethods() []CheckMethod {
	return []CheckMethod{
		CheckMethodCommand,
		CheckMethodObservation,
		CheckMethodManual,
		CheckMethodRegression,
	}
}

func (m CheckMethod) Valid() bool {
	return slices.Contains(CheckMethods(), m)
}

func (m CheckMethod) NeedsAttestation() bool {
	return m == CheckMethodManual
}

func (m CheckMethod) NeedsBothDirections() bool {
	return m == CheckMethodRegression
}

type CheckApproval string

const (
	CheckApprovalPending  CheckApproval = "pending"
	CheckApprovalApproved CheckApproval = "approved"
	CheckApprovalDeclined CheckApproval = "declined"
)

func CheckApprovals() []CheckApproval {
	return []CheckApproval{CheckApprovalPending, CheckApprovalApproved, CheckApprovalDeclined}
}

func (a CheckApproval) Valid() bool {
	return slices.Contains(CheckApprovals(), a)
}

func (a CheckApproval) Decided() bool {
	return a == CheckApprovalApproved || a == CheckApprovalDeclined
}

func CheckApprovalOn(actor Actor) CheckApproval {
	if actor.Kind == ActorKindUser {
		return CheckApprovalApproved
	}

	return CheckApprovalPending
}

type CheckResolution string

const (
	CheckResolutionNone   CheckResolution = "none"
	CheckResolutionWaived CheckResolution = "waived"
	CheckResolutionGap    CheckResolution = "gap"
)

func CheckResolutions() []CheckResolution {
	return []CheckResolution{CheckResolutionNone, CheckResolutionWaived, CheckResolutionGap}
}

func (r CheckResolution) Valid() bool {
	return slices.Contains(CheckResolutions(), r)
}

type Check struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	IssueID              uuid.UUID
	Position             int
	Statement            string
	Method               CheckMethod
	Proof                string
	TimeLimit            *time.Duration
	Approval             CheckApproval
	ApprovedByAccountID  uuid.UUID
	ApprovedAt           *time.Time
	Resolution           CheckResolution
	ResolutionReason     string
	ResolvedByAccountID  uuid.UUID
	ResolvedAt           *time.Time
	GapIssueID           uuid.UUID
	AuthorKind           ActorKind
	CreatedByAccountID   uuid.UUID
	AddedAfterDelegation bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (c Check) Resolved() bool {
	return c.Resolution != CheckResolutionNone
}

func GapIssueTitle(statement string) string {
	title := "Gap: " + strings.TrimSpace(statement)

	if utf8.RuneCountInString(title) <= IssueTitleMaxLen {
		return title
	}

	return string([]rune(title)[:IssueTitleMaxLen-1]) + "…"
}

func CheckStatements(checks []Check) string {
	statements := make([]string, 0, len(checks))

	for _, check := range checks {
		statements = append(statements, check.Statement)
	}

	return strings.Join(statements, "; ")
}

func ValidateCheckStatement(field, statement string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(statement))

	switch {
	case length < CheckStatementMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > CheckStatementMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateCheckProof(field, proof string) FieldError {
	trimmed := strings.TrimSpace(proof)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > CheckProofMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateCheckMethod(field string, method CheckMethod) FieldError {
	if !method.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateCheckTimeLimit(field string, limit *time.Duration) FieldError {
	if limit == nil {
		return FieldError{}
	}

	if *limit < CheckTimeLimitMin || *limit > CheckTimeLimitMax {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}

func ValidateCheckReason(field, reason string) FieldError {
	trimmed := strings.TrimSpace(reason)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > CheckReasonMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}
