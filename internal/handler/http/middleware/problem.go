package middleware

import (
	"encoding/json"
	"net/http"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const (
	problemContentType = "application/problem+json"
	problemType        = "about:blank"
)

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	problem := api.Problem{
		Type:   problemType,
		Title:  http.StatusText(status),
		Status: int32(status),
	}

	if detail != "" {
		problem.Detail = &detail
	}

	if correlationID, ok := CorrelationIDFrom(r.Context()); ok {
		problem.Instance = &correlationID
	}

	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(problem)
}
