package entity

import (
	"sort"
	"strings"
)

const (
	ValidationCodeRequired         = "required"
	ValidationCodeTooShort         = "too_short"
	ValidationCodeTooLong          = "too_long"
	ValidationCodeMalformed        = "malformed"
	ValidationCodeUnknownTimezone  = "unknown_timezone"
	ValidationCodeUnsupportedValue = "unsupported_value"
	ValidationCodeBreached         = "breached"
	ValidationCodeReused           = "reused"
)

type FieldError struct {
	Field string
	Code  string
}

type ValidationError struct {
	Fields []FieldError
}

func (e ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, field := range e.Fields {
		parts[i] = field.Field + ": " + field.Code
	}

	sort.Strings(parts)

	return "validation failed: " + strings.Join(parts, ", ")
}

func NewValidationError(fields ...FieldError) error {
	present := make([]FieldError, 0, len(fields))

	for _, field := range fields {
		if field.Field != "" && field.Code != "" {
			present = append(present, field)
		}
	}

	if len(present) == 0 {
		return nil
	}

	return ValidationError{Fields: present}
}
