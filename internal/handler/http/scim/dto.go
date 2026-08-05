package scim

import (
	"time"

	"github.com/usenorn/norn/internal/entity"
)

const (
	userSchema    = "urn:ietf:params:scim:schemas:core:2.0:User"
	groupSchema   = "urn:ietf:params:scim:schemas:core:2.0:Group"
	listSchema    = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	patchSchema   = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	errorSchema   = "urn:ietf:params:scim:api:messages:2.0:Error"
	configSchema  = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	resourceTypes = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
)

type meta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location"`
}

type email struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type name struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type userResource struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	ExternalID  string            `json:"externalId,omitempty"`
	UserName    string            `json:"userName"`
	DisplayName string            `json:"displayName,omitempty"`
	Name        *name             `json:"name,omitempty"`
	Emails      []email           `json:"emails,omitempty"`
	Active      bool              `json:"active"`
	Meta        meta              `json:"meta"`
	Norn        *departureSummary `json:"urn:norn:params:scim:extension:departure,omitempty"`
}

type departureSummary struct {
	Issues   []string `json:"openIssues,omitempty"`
	Projects []string `json:"ledProjects,omitempty"`
	Teams    []string `json:"teams,omitempty"`
}

type groupMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

type groupResource struct {
	Schemas     []string      `json:"schemas"`
	ID          string        `json:"id"`
	ExternalID  string        `json:"externalId,omitempty"`
	DisplayName string        `json:"displayName"`
	Members     []groupMember `json:"members"`
	Meta        meta          `json:"meta"`
}

type listResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Resources    []any    `json:"Resources"`
}

type errorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	SCIMType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

type patchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type patchRequest struct {
	Schemas    []string         `json:"schemas"`
	Operations []patchOperation `json:"Operations"`
}

func userDTO(user entity.DirectoryUser, base string, departure *entity.DirectoryWorkload) userResource {
	resource := userResource{
		Schemas:     []string{userSchema},
		ID:          user.ID.String(),
		ExternalID:  user.ExternalID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Emails:      []email{{Value: user.UserName, Primary: true}},
		Active:      user.Active,
		Meta: meta{
			ResourceType: "User",
			Created:      user.CreatedAt.UTC(),
			LastModified: user.UpdatedAt.UTC(),
			Location:     base + "/Users/" + user.ID.String(),
		},
	}

	if departure != nil && !departure.Empty() {
		resource.Norn = &departureSummary{
			Issues:   departure.Issues,
			Projects: departure.Projects,
			Teams:    departure.Teams,
		}
	}

	return resource
}

func groupDTO(group entity.DirectoryGroup, members []entity.DirectoryUser, base string) groupResource {
	resource := groupResource{
		Schemas:     []string{groupSchema},
		ID:          group.ID.String(),
		ExternalID:  group.ExternalID,
		DisplayName: group.DisplayName,
		Members:     make([]groupMember, 0, len(members)),
		Meta: meta{
			ResourceType: "Group",
			Created:      group.CreatedAt.UTC(),
			LastModified: group.UpdatedAt.UTC(),
			Location:     base + "/Groups/" + group.ID.String(),
		},
	}

	for _, member := range members {
		resource.Members = append(resource.Members, groupMember{
			Value:   member.ID.String(),
			Display: member.UserName,
		})
	}

	return resource
}
