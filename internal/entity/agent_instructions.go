package entity

import (
	"strings"
	"unicode/utf8"
)

const AgentInstructionsMaxLen = 20000

func NormaliseAgentInstructions(instructions string) string {
	return strings.TrimSpace(instructions)
}

func ValidateAgentInstructions(field, instructions string) FieldError {
	normalised := NormaliseAgentInstructions(instructions)

	switch {
	case strings.ContainsRune(normalised, 0):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	case utf8.RuneCountInString(normalised) > AgentInstructionsMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ComposeAgentInstructions(workspace, project, agent string) string {
	var written []string

	for _, level := range []string{workspace, project, agent} {
		if normalised := NormaliseAgentInstructions(level); normalised != "" {
			written = append(written, normalised)
		}
	}

	return strings.Join(written, "\n\n")
}
