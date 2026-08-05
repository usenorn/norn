package auditexport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
)

const (
	WorkspacePath = "/v1/workspaces/{workspaceId}/audit/export"
	InstancePath  = "/v1/instance/audit/export"
	contentType   = "application/x-ndjson"
)

type Edge struct {
	audit service.AuditLog
}

func New(audit service.AuditLog) *Edge {
	return &Edge{audit: audit}
}

func (e *Edge) ServeWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := uuid.Parse(r.PathValue("workspaceId"))
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusBadRequest, "workspace id is not a uuid")

		return
	}

	e.stream(w, r, service.AuditScope{WorkspaceID: workspaceID}, "norn-audit.ndjson")
}

func (e *Edge) ServeInstance(w http.ResponseWriter, r *http.Request) {
	e.stream(w, r, service.AuditScope{Instance: true}, "norn-audit-instance.ndjson")
}

func (e *Edge) stream(
	w http.ResponseWriter,
	r *http.Request,
	scope service.AuditScope,
	filename string,
) {
	input, err := inputFrom(r)
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusUnprocessableEntity, err.Error())

		return
	}

	stream := http.NewResponseController(w)
	_ = stream.SetWriteDeadline(time.Time{})

	encoder := json.NewEncoder(w)
	started := false

	emit := func(entry entity.AuditEntry) error {
		if !started {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)

			started = true
		}

		if err := encoder.Encode(exported(entry)); err != nil {
			return err
		}

		return stream.Flush()
	}

	if err := e.audit.Export(r.Context(), scope, input, emit); err != nil {
		if started {
			return
		}

		status, detail := problem(err)
		middleware.WriteProblem(w, r, status, detail)

		return
	}

	if !started {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	}
}

func inputFrom(r *http.Request) (service.ListAuditInput, error) {
	query := r.URL.Query()
	filter := entity.AuditFilter{ResourceKind: query.Get("resourceKind")}

	if raw := query.Get("actor"); raw != "" {
		actor, err := uuid.Parse(raw)
		if err != nil {
			return service.ListAuditInput{}, errors.New("actor is not a uuid")
		}

		filter.ActorAccountID = actor
	}

	if raw := query.Get("resourceId"); raw != "" {
		resource, err := uuid.Parse(raw)
		if err != nil {
			return service.ListAuditInput{}, errors.New("resourceId is not a uuid")
		}

		filter.ResourceID = resource
	}

	for _, raw := range query["action"] {
		action := entity.AuditAction(raw)
		if !action.Valid() {
			return service.ListAuditInput{}, errors.New("action is not one this instance records")
		}

		filter.Actions = append(filter.Actions, action)
	}

	from, err := boundary(query.Get("from"))
	if err != nil {
		return service.ListAuditInput{}, err
	}

	to, err := boundary(query.Get("to"))
	if err != nil {
		return service.ListAuditInput{}, err
	}

	filter.From = from
	filter.To = to

	return service.ListAuditInput{Filter: filter}, nil
}

func boundary(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("date range must be RFC 3339")
	}

	return &at, nil
}
