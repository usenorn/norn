package scim

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

var filterPattern = regexp.MustCompile(`(?i)^\s*(\w+)\s+eq\s+"([^"]*)"\s*$`)

func filterValue(filter, attribute string) (string, error) {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return "", nil
	}

	match := filterPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", entity.ErrDirectoryPatchUnsupported
	}

	if !strings.EqualFold(match[1], attribute) && !strings.EqualFold(match[1], "externalId") {
		return "", entity.ErrDirectoryPatchUnsupported
	}

	return match[2], nil
}

func profileOf(resource userResource) service.DirectoryProfile {
	profile := service.DirectoryProfile{
		ExternalID:  strings.TrimSpace(resource.ExternalID),
		UserName:    strings.TrimSpace(resource.UserName),
		DisplayName: strings.TrimSpace(resource.DisplayName),
		Active:      resource.Active,
	}

	if profile.UserName == "" {
		for _, address := range resource.Emails {
			if address.Primary && strings.TrimSpace(address.Value) != "" {
				profile.UserName = strings.TrimSpace(address.Value)

				break
			}
		}
	}

	if profile.DisplayName == "" && resource.Name != nil {
		profile.DisplayName = strings.TrimSpace(resource.Name.Formatted)

		if profile.DisplayName == "" {
			profile.DisplayName = strings.TrimSpace(
				resource.Name.GivenName + " " + resource.Name.FamilyName,
			)
		}
	}

	return profile
}

func groupProfileOf(resource groupResource) (service.DirectoryGroupProfile, error) {
	profile := service.DirectoryGroupProfile{
		ExternalID:  strings.TrimSpace(resource.ExternalID),
		DisplayName: strings.TrimSpace(resource.DisplayName),
	}

	for _, member := range resource.Members {
		id, err := uuid.Parse(strings.TrimSpace(member.Value))
		if err != nil {
			return service.DirectoryGroupProfile{}, entity.ErrDirectoryUserNotFound
		}

		profile.Members = append(profile.Members, id)
	}

	return profile, nil
}

func activeFrom(request patchRequest) (bool, error) {
	for _, operation := range request.Operations {
		if !strings.EqualFold(operation.Op, "replace") && !strings.EqualFold(operation.Op, "add") {
			continue
		}

		path := strings.TrimSpace(strings.ToLower(operation.Path))

		if path == "active" {
			return truthy(operation.Value)
		}

		if path == "" {
			attributes, ok := operation.Value.(map[string]any)
			if !ok {
				continue
			}

			for key, value := range attributes {
				if strings.EqualFold(key, "active") {
					return truthy(value)
				}
			}
		}
	}

	return false, entity.ErrDirectoryPatchUnsupported
}

func truthy(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true"), nil
	default:
		return false, entity.ErrDirectoryPatchUnsupported
	}
}

func membershipEdit(request patchRequest) (service.DirectoryMembershipEdit, error) {
	var edit service.DirectoryMembershipEdit

	for _, operation := range request.Operations {
		path := strings.TrimSpace(strings.ToLower(operation.Path))

		if path != "members" && path != "" {
			return service.DirectoryMembershipEdit{}, entity.ErrDirectoryPatchUnsupported
		}

		ids, err := memberIDs(operation.Value)
		if err != nil {
			return service.DirectoryMembershipEdit{}, err
		}

		switch strings.ToLower(strings.TrimSpace(operation.Op)) {
		case "add", "replace":
			edit.Add = append(edit.Add, ids...)
		case "remove":
			edit.Remove = append(edit.Remove, ids...)
		default:
			return service.DirectoryMembershipEdit{}, entity.ErrDirectoryPatchUnsupported
		}
	}

	if len(edit.Add) == 0 && len(edit.Remove) == 0 {
		return service.DirectoryMembershipEdit{}, entity.ErrDirectoryPatchUnsupported
	}

	return edit, nil
}

func memberIDs(value any) ([]uuid.UUID, error) {
	entries, ok := value.([]any)
	if !ok {
		return nil, entity.ErrDirectoryPatchUnsupported
	}

	ids := make([]uuid.UUID, 0, len(entries))

	for _, entry := range entries {
		member, ok := entry.(map[string]any)
		if !ok {
			return nil, entity.ErrDirectoryPatchUnsupported
		}

		raw, ok := member["value"].(string)
		if !ok {
			return nil, entity.ErrDirectoryPatchUnsupported
		}

		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, errors.Join(entity.ErrDirectoryUserNotFound, err)
		}

		ids = append(ids, id)
	}

	return ids, nil
}
