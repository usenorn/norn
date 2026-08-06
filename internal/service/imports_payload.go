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

// ImportCyclePayload carries Name and Number so the report can say what was lost: a Norn
// cycle has no name, and its number is the team's own running count rather than anything a
// source chooses. EndsOn is the last day the cycle covers — an adapter whose source ends a
// cycle on the day after converts it, because workspace_cycles excludes overlaps on
// daterange(starts_on, ends_on, '[]') and one day of slack makes consecutive cycles collide.
type ImportCyclePayload struct {
	Team     string `json:"team"`
	Name     string `json:"name"`
	Number   int    `json:"number"`
	StartsOn string `json:"startsOn"`
	EndsOn   string `json:"endsOn"`
	ClosedOn string `json:"closedOn"`
}

// Defect is how an adapter hands over a row it could not read. Fetch returning an error kills
// the whole run, so a row a source cannot parse is staged carrying its own explanation instead
// and costs nothing but itself.
type ImportIssuePayload struct {
	Defect      string     `json:"defect"`
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

// ImportAttachmentPayload names an object the adapter has already put in blob storage under
// entity.ImportBlobKey, while the source's signed URL was still alive. The apply pass adopts
// the object it names and fetches nothing, which is why the size and the type travel with it;
// SourceURL is kept so the report can say where a file that could not be stored came from.
// Inline marks a file a body points at rather than one hanging off the row, and it is what
// the staging pass derives an embed record from.
type ImportAttachmentPayload struct {
	Issue       string `json:"issue"`
	Comment     string `json:"comment"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	ObjectKey   string `json:"objectKey"`
	SourceURL   string `json:"sourceUrl"`
	Inline      bool   `json:"inline"`
}

type ImportEmbedPayload struct {
	Issue       string   `json:"issue"`
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
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
