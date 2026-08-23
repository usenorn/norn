package entity

import (
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	ExecutionServiceNameMaxLen   = 32
	ExecutionServiceReasonMaxLen = 1000
	ExecutionServicesMax         = 32

	ExecutionServicePortMin = 1
	ExecutionServicePortMax = 65535
)

var (
	ErrExecutionServiceStale = errors.New(
		"a newer report is already on record for this service",
	)
	ErrExecutionServiceCrowded = errors.New(
		"this run already holds as many services as it may",
	)
)

var executionServiceName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

type ExecutionServiceState string

const (
	ExecutionServiceStarting  ExecutionServiceState = channelv1.ServiceStarting
	ExecutionServiceHealthy   ExecutionServiceState = channelv1.ServiceHealthy
	ExecutionServiceUnhealthy ExecutionServiceState = channelv1.ServiceUnhealthy
	ExecutionServiceStopped   ExecutionServiceState = channelv1.ServiceStopped
)

func ExecutionServiceStates() []ExecutionServiceState {
	return []ExecutionServiceState{
		ExecutionServiceStarting,
		ExecutionServiceHealthy,
		ExecutionServiceUnhealthy,
		ExecutionServiceStopped,
	}
}

func (s ExecutionServiceState) Valid() bool {
	return slices.Contains(ExecutionServiceStates(), s)
}

func (s ExecutionServiceState) Live() bool {
	return s == ExecutionServiceStarting || s == ExecutionServiceHealthy
}

type ExecutionServiceProbe string

const (
	ExecutionServiceProbeNone ExecutionServiceProbe = channelv1.ProbeNone
	ExecutionServiceProbeHTTP ExecutionServiceProbe = channelv1.ProbeHTTP
	ExecutionServiceProbeTCP  ExecutionServiceProbe = channelv1.ProbeTCP
	ExecutionServiceProbeLog  ExecutionServiceProbe = channelv1.ProbeLog
)

func ExecutionServiceProbes() []ExecutionServiceProbe {
	return []ExecutionServiceProbe{
		ExecutionServiceProbeNone,
		ExecutionServiceProbeHTTP,
		ExecutionServiceProbeTCP,
		ExecutionServiceProbeLog,
	}
}

func (p ExecutionServiceProbe) Valid() bool {
	return slices.Contains(ExecutionServiceProbes(), p)
}

type ExecutionService struct {
	ID          uuid.UUID
	ExecutionID string
	WorkspaceID uuid.UUID
	Name        string
	State       ExecutionServiceState
	Probe       ExecutionServiceProbe
	Port        int
	Reason      string
	ReportedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s ExecutionService) Live() bool {
	return s.State.Live()
}

func ValidateExecutionServiceName(field, name string) FieldError {
	switch {
	case name == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(name) > ExecutionServiceNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !executionServiceName.MatchString(name):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

func ValidateExecutionService(field string, service ExecutionService) error {
	state := FieldError{}
	if !service.State.Valid() {
		state = FieldError{Field: field + ".state", Code: ValidationCodeUnsupportedValue}
	}

	probe := FieldError{}
	if !service.Probe.Valid() {
		probe = FieldError{Field: field + ".probe", Code: ValidationCodeUnsupportedValue}
	}

	port := FieldError{}
	if service.Port != 0 &&
		(service.Port < ExecutionServicePortMin || service.Port > ExecutionServicePortMax) {
		port = FieldError{Field: field + ".port", Code: ValidationCodeOutOfRange}
	}

	return NewValidationError(
		ValidateExecutionServiceName(field+".name", service.Name),
		optionalText(field+".reason", service.Reason, ExecutionServiceReasonMaxLen),
		state,
		probe,
		port,
	)
}
