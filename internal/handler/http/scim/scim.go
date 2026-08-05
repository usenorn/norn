package scim

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	BasePath      = "/v1/scim/v2"
	UsersPath     = BasePath + "/Users"
	UserPath      = BasePath + "/Users/{id}"
	GroupsPath    = BasePath + "/Groups"
	GroupPath     = BasePath + "/Groups/{id}"
	ConfigPath    = BasePath + "/ServiceProviderConfig"
	TypesPath     = BasePath + "/ResourceTypes"
	SchemasPath   = BasePath + "/Schemas"
	contentType   = "application/scim+json"
	bearerPrefix  = "Bearer "
	maxRequestLen = 1 << 20
)

type Edge struct {
	directories service.Directories
}

func New(directories service.Directories) *Edge {
	return &Edge{directories: directories}
}

func (e *Edge) connection(w http.ResponseWriter, r *http.Request) (entity.DirectoryConnection, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearerPrefix) {
		writeError(w, http.StatusUnauthorized, "", "a directory credential is required")

		return entity.DirectoryConnection{}, false
	}

	connection, err := e.directories.Authenticate(
		r.Context(), strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix)),
	)
	if err != nil {
		status, scimType, detail := problem(err)
		writeError(w, status, scimType, detail)

		return entity.DirectoryConnection{}, false
	}

	return connection, true
}

func (e *Edge) CreateUser(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	var resource userResource

	if !decode(w, r, &resource) {
		return
	}

	view, err := e.directories.PutUser(r.Context(), connection.WorkspaceID, nil, profileOf(resource))
	if err != nil {
		fail(w, err)

		return
	}

	write(w, http.StatusCreated, userDTO(view.User, BasePath, nil))
}

func (e *Edge) ReplaceUser(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	var resource userResource

	if !decode(w, r, &resource) {
		return
	}

	view, err := e.directories.PutUser(r.Context(), connection.WorkspaceID, &id, profileOf(resource))
	if err != nil {
		fail(w, err)

		return
	}

	write(w, http.StatusOK, userDTO(view.User, BasePath, &view.Workload))
}

func (e *Edge) PatchUser(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	var request patchRequest

	if !decode(w, r, &request) {
		return
	}

	active, err := activeFrom(request)
	if err != nil {
		fail(w, err)

		return
	}

	view, err := e.directories.SetUserActive(r.Context(), connection.WorkspaceID, id, active)
	if err != nil {
		fail(w, err)

		return
	}

	write(w, http.StatusOK, userDTO(view.User, BasePath, &view.Workload))
}

func (e *Edge) GetUser(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	user, err := e.directories.GetUser(r.Context(), connection.WorkspaceID, id)
	if err != nil {
		fail(w, err)

		return
	}

	write(w, http.StatusOK, userDTO(user, BasePath, nil))
}

func (e *Edge) ListUsers(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	userName, err := filterValue(r.URL.Query().Get("filter"), "userName")
	if err != nil {
		fail(w, err)

		return
	}

	users, err := e.directories.ListUsers(
		r.Context(), connection.WorkspaceID, userName, count(r),
	)
	if err != nil {
		fail(w, err)

		return
	}

	resources := make([]any, 0, len(users))
	for _, user := range users {
		resources = append(resources, userDTO(user, BasePath, nil))
	}

	write(w, http.StatusOK, page(resources))
}

func (e *Edge) DeleteUser(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	if _, err := e.directories.DeleteUser(r.Context(), connection.WorkspaceID, id); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (e *Edge) CreateGroup(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	var resource groupResource

	if !decode(w, r, &resource) {
		return
	}

	profile, err := groupProfileOf(resource)
	if err != nil {
		fail(w, err)

		return
	}

	group, err := e.directories.PutGroup(r.Context(), connection.WorkspaceID, nil, profile)
	if err != nil {
		fail(w, err)

		return
	}

	e.writeGroup(w, r, connection, group, http.StatusCreated)
}

func (e *Edge) ReplaceGroup(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	var resource groupResource

	if !decode(w, r, &resource) {
		return
	}

	profile, err := groupProfileOf(resource)
	if err != nil {
		fail(w, err)

		return
	}

	group, err := e.directories.PutGroup(r.Context(), connection.WorkspaceID, &id, profile)
	if err != nil {
		fail(w, err)

		return
	}

	e.writeGroup(w, r, connection, group, http.StatusOK)
}

func (e *Edge) PatchGroup(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	var request patchRequest

	if !decode(w, r, &request) {
		return
	}

	edit, err := membershipEdit(request)
	if err != nil {
		fail(w, err)

		return
	}

	group, err := e.directories.EditGroupMembers(r.Context(), connection.WorkspaceID, id, edit)
	if err != nil {
		fail(w, err)

		return
	}

	e.writeGroup(w, r, connection, group, http.StatusOK)
}

func (e *Edge) GetGroup(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	group, err := e.directories.GetGroup(r.Context(), connection.WorkspaceID, id)
	if err != nil {
		fail(w, err)

		return
	}

	e.writeGroup(w, r, connection, group, http.StatusOK)
}

func (e *Edge) ListGroups(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	displayName, err := filterValue(r.URL.Query().Get("filter"), "displayName")
	if err != nil {
		fail(w, err)

		return
	}

	groups, err := e.directories.ListGroups(
		r.Context(), connection.WorkspaceID, displayName, count(r),
	)
	if err != nil {
		fail(w, err)

		return
	}

	resources := make([]any, 0, len(groups))

	for _, group := range groups {
		members, err := e.directories.ListGroupMembers(r.Context(), connection.WorkspaceID, group.ID)
		if err != nil {
			fail(w, err)

			return
		}

		resources = append(resources, groupDTO(group, members, BasePath))
	}

	write(w, http.StatusOK, page(resources))
}

func (e *Edge) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	connection, ok := e.connection(w, r)
	if !ok {
		return
	}

	id, ok := identifier(w, r)
	if !ok {
		return
	}

	if err := e.directories.DeleteGroup(r.Context(), connection.WorkspaceID, id); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (e *Edge) writeGroup(
	w http.ResponseWriter,
	r *http.Request,
	connection entity.DirectoryConnection,
	group entity.DirectoryGroup,
	status int,
) {
	members, err := e.directories.ListGroupMembers(r.Context(), connection.WorkspaceID, group.ID)
	if err != nil {
		fail(w, err)

		return
	}

	write(w, status, groupDTO(group, members, BasePath))
}

func identifier(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "invalidValue", "that resource id is not one Norn issued")

		return uuid.Nil, false
	}

	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	body := http.MaxBytesReader(w, r.Body, maxRequestLen)

	if err := json.NewDecoder(body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalidSyntax", "the request body is not valid JSON")

		return false
	}

	return true
}

func count(r *http.Request) int {
	requested, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || requested <= 0 {
		return entity.DirectoryPageMaxSize
	}

	return requested
}

func page(resources []any) listResponse {
	return listResponse{
		Schemas:      []string{listSchema},
		TotalResults: len(resources),
		ItemsPerPage: len(resources),
		StartIndex:   1,
		Resources:    resources,
	}
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, err error) {
	status, scimType, detail := problem(err)
	writeError(w, status, scimType, detail)
}

func writeError(w http.ResponseWriter, status int, scimType, detail string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(errorResponse{
		Schemas:  []string{errorSchema},
		Status:   strconv.Itoa(status),
		SCIMType: scimType,
		Detail:   detail,
	})
}
