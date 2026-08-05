package scim

import "net/http"

func (e *Edge) ServiceProviderConfig(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, map[string]any{
		"schemas":          []string{configSchema},
		"documentationUri": "https://norn.dev/docs/directory",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": 200},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": false},
		"authenticationSchemes": []any{map[string]any{
			"type":        "oauthbearertoken",
			"name":        "OAuth Bearer Token",
			"description": "Authentication with the token Norn issued for this workspace",
			"primary":     true,
		}},
		"meta": map[string]any{
			"resourceType": "ServiceProviderConfig",
			"location":     BasePath + "/ServiceProviderConfig",
		},
	})
}

func (e *Edge) ResourceTypes(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, page([]any{
		map[string]any{
			"schemas":     []string{resourceTypes},
			"id":          "User",
			"name":        "User",
			"endpoint":    "/Users",
			"description": "A person the directory provisions into this workspace",
			"schema":      userSchema,
			"meta": map[string]any{
				"resourceType": "ResourceType",
				"location":     BasePath + "/ResourceTypes/User",
			},
		},
		map[string]any{
			"schemas":     []string{resourceTypes},
			"id":          "Group",
			"name":        "Group",
			"endpoint":    "/Groups",
			"description": "A directory group mapped onto a Norn team",
			"schema":      groupSchema,
			"meta": map[string]any{
				"resourceType": "ResourceType",
				"location":     BasePath + "/ResourceTypes/Group",
			},
		},
	}))
}

func (e *Edge) Schemas(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, page([]any{
		map[string]any{
			"id":          userSchema,
			"name":        "User",
			"description": "SCIM core User",
			"attributes": []any{
				attribute("userName", "string", true, true),
				attribute("displayName", "string", false, false),
				attribute("active", "boolean", false, false),
				attribute("externalId", "string", false, false),
			},
			"meta": map[string]any{"resourceType": "Schema", "location": BasePath + "/Schemas/" + userSchema},
		},
		map[string]any{
			"id":          groupSchema,
			"name":        "Group",
			"description": "SCIM core Group",
			"attributes": []any{
				attribute("displayName", "string", true, true),
				attribute("members", "complex", false, false),
			},
			"meta": map[string]any{"resourceType": "Schema", "location": BasePath + "/Schemas/" + groupSchema},
		},
	}))
}

func attribute(name, kind string, required, unique bool) map[string]any {
	uniqueness := "none"
	if unique {
		uniqueness = "server"
	}

	return map[string]any{
		"name":        name,
		"type":        kind,
		"multiValued": kind == "complex",
		"required":    required,
		"caseExact":   false,
		"mutability":  "readWrite",
		"returned":    "default",
		"uniqueness":  uniqueness,
	}
}
