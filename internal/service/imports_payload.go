package service

// These are the shapes an adapter renders a foreign tracker into. They are Norn's
// vocabulary rather than any source's: an adapter's whole job is the translation, which
// is why nothing here names a product. Every reference to another thing is that thing's
// identifier in the source, resolved through the ledger and the mapping plan at apply
// time — an adapter never sees a Norn identifier.

type ImportTeamPayload struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ImportWorkflowStatePayload struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Team     string `json:"team"`
}

type ImportLabelGroupPayload struct {
	Name string `json:"name"`
}

type ImportLabelPayload struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Group string `json:"group"`
	Team  string `json:"team"`
}

type ImportProjectPayload struct {
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Lead        ImportUser `json:"lead"`
}

type ImportIssuePayload struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Team        string     `json:"team"`
	State       string     `json:"state"`
	Priority    string     `json:"priority"`
	Project     string     `json:"project"`
	Labels      []string   `json:"labels"`
	Assignee    ImportUser `json:"assignee"`
	Author      ImportUser `json:"author"`
	Estimate    int        `json:"estimate"`
	DueOn       string     `json:"dueOn"`
	Cycle       string     `json:"cycle"`
}

type ImportIssueParentPayload struct {
	Issue  string `json:"issue"`
	Parent string `json:"parent"`
}

type ImportIssueRelationPayload struct {
	Issue   string `json:"issue"`
	Related string `json:"related"`
	Kind    string `json:"kind"`
}

type ImportCommentPayload struct {
	Issue  string     `json:"issue"`
	Body   string     `json:"body"`
	Author ImportUser `json:"author"`
	Parent string     `json:"parent"`
}

// ImportUser travels with every reference to a person rather than being staged as a
// resource of its own, because an import never creates an account. Key is what the
// mapping plan is keyed on; Name and Email exist only so a match can be suggested.
type ImportUser struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u ImportUser) Named() bool { return u.Key != "" }
