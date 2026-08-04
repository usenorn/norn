package label_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func TestALabelIsOnlyEverReferencedByID(t *testing.T) {
	selectors := map[string]reflect.Type{
		"UpdateWorkspaceLabelRequestObject":      reflect.TypeOf(api.UpdateWorkspaceLabelRequestObject{}),
		"RemoveWorkspaceLabelRequestObject":      reflect.TypeOf(api.RemoveWorkspaceLabelRequestObject{}),
		"GetWorkspaceLabelUsageRequestObject":    reflect.TypeOf(api.GetWorkspaceLabelUsageRequestObject{}),
		"MergeWorkspaceLabelRequestObject":       reflect.TypeOf(api.MergeWorkspaceLabelRequestObject{}),
		"MergeLabelRequest":                      reflect.TypeOf(api.MergeLabelRequest{}),
		"CreateLabelRequest":                     reflect.TypeOf(api.CreateLabelRequest{}),
		"UpdateLabelRequest":                     reflect.TypeOf(api.UpdateLabelRequest{}),
		"SetIssueLabelsRequest":                  reflect.TypeOf(api.SetIssueLabelsRequest{}),
		"SetWorkspaceIssueLabelsRequestObject":   reflect.TypeOf(api.SetWorkspaceIssueLabelsRequestObject{}),
		"RenameWorkspaceLabelGroupRequestObject": reflect.TypeOf(api.RenameWorkspaceLabelGroupRequestObject{}),
		"RemoveWorkspaceLabelGroupRequestObject": reflect.TypeOf(api.RemoveWorkspaceLabelGroupRequestObject{}),
	}

	id := reflect.TypeOf(uuid.UUID{})

	var walk func(t *testing.T, owner string, path string, typ reflect.Type)

	walk = func(t *testing.T, owner, path string, typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}

		if typ.Kind() != reflect.Struct || typ == id {
			return
		}

		for i := range typ.NumField() {
			field := typ.Field(i)
			where := path + "." + field.Name

			names := strings.ToLower(field.Name)
			if strings.Contains(names, "label") || strings.Contains(names, "group") {
				kind := field.Type
				for kind.Kind() == reflect.Pointer || kind.Kind() == reflect.Slice {
					kind = kind.Elem()
				}

				if kind.Kind() == reflect.String && kind != id {
					t.Errorf(
						"%s selects a label by %s, a string — a rename would then break every filter and "+
							"saved reference pointing at it. Only workspace_issue_activity may freeze a name, "+
							"because history must not be rewritten by a rename.",
						owner, where,
					)
				}
			}

			walk(t, owner, where, field.Type)
		}
	}

	for name, typ := range selectors {
		walk(t, name, name, typ)
	}
}

func TestALabelResponseCarriesItsIDSoAReaderCanReferToItLater(t *testing.T) {
	label := reflect.TypeOf(api.Label{})

	field, ok := label.FieldByName("Id")
	if !ok {
		t.Fatal("api.Label has no Id — nothing could refer to a label across a rename")
	}

	if field.Type != reflect.TypeOf(uuid.UUID{}) {
		t.Fatalf("api.Label.Id is %s, want uuid.UUID", field.Type)
	}
}
