package mcpserver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type searchDocsInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Query     string `json:"query" jsonschema:"words from a doc's title or body"`
	Project   string `json:"project,omitempty" jsonschema:"only docs shared with this project, by slug or id"`
	Limit     int    `json:"limit,omitempty" jsonschema:"at most 25 docs; 10 when omitted"`
}

type listDocsInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Project   string `json:"project,omitempty" jsonschema:"only docs shared with this project, by slug or id"`
	Under     string `json:"under,omitempty" jsonschema:"a doc id; lists every doc beneath it, at any depth"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"the cursor returned by the previous page"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, at most 200"`
}

type getDocInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Doc       string `json:"doc" jsonschema:"the doc id"`
}

type createDocInput struct {
	Workspace  string `json:"workspace" jsonschema:"the workspace slug or id"`
	Title      string `json:"title" jsonschema:"the doc's title"`
	Markdown   string `json:"markdown" jsonschema:"the doc's body, markdown"`
	Parent     string `json:"parent,omitempty" jsonschema:"a doc id to file the new doc beneath"`
	Visibility string `json:"visibility,omitempty" jsonschema:"who may read it: workspace, project, or personal; project when a project is named, workspace otherwise"`
	Project    string `json:"project,omitempty" jsonschema:"the project whose members may read it, by slug or id; needed for project visibility"`
}

type editDocInput struct {
	Workspace       string  `json:"workspace" jsonschema:"the workspace slug or id"`
	Doc             string  `json:"doc" jsonschema:"the doc id"`
	ExpectedVersion int     `json:"expected_version" jsonschema:"the version norn_get_doc returned with the text this edit was written against"`
	Title           *string `json:"title,omitempty" jsonschema:"a new title"`
	Markdown        *string `json:"markdown,omitempty" jsonschema:"the new body, markdown; with section, the new text of that section including its heading"`
	Section         *string `json:"section,omitempty" jsonschema:"the exact text of a heading; replaces that heading and everything beneath it up to the next heading of the same or a higher level"`
}

type linkDocInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Doc       string `json:"doc" jsonschema:"the doc id"`
	Issue     string `json:"issue,omitempty" jsonschema:"the issue reference like ENG-42, or its id; name an issue or a project, not both"`
	Project   string `json:"project,omitempty" jsonschema:"the project slug or id; name an issue or a project, not both"`
}

type docOutput struct {
	Doc docDTO `json:"doc"`
}

type linkedDocOutput struct {
	Doc   docDTO       `json:"doc"`
	Links []docLinkDTO `json:"links"`
}

type docLinkOutput struct {
	Link docLinkDTO `json:"link"`
}

type docListOutput struct {
	Docs       []docSummaryDTO `json:"docs"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type docSearchOutput struct {
	Docs []docMatchDTO `json:"docs"`
}

func (t *toolset) searchDocs(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input searchDocsInput,
) (*mcp.CallToolResult, docSearchOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docSearchOutput{}, toolFailure(ctx, err)
	}

	projectID, err := t.optionalProject(ctx, workspace.ID, input.Project)
	if err != nil {
		return nil, docSearchOutput{}, toolFailure(ctx, err)
	}

	matches, err := t.docs.Search(ctx, workspace.ID, service.SearchDocsInput{
		Query:     input.Query,
		ProjectID: projectID,
		Limit:     input.Limit,
	})
	if err != nil {
		return nil, docSearchOutput{}, toolFailure(ctx, err)
	}

	output := docSearchOutput{Docs: make([]docMatchDTO, 0, len(matches))}

	for _, match := range matches {
		output.Docs = append(output.Docs, docMatchDTO{
			docSummaryDTO: docSummaryDTOFrom(match.DocSummary),
			Excerpt:       match.Excerpt,
		})
	}

	return nil, output, nil
}

func (t *toolset) listDocs(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listDocsInput,
) (*mcp.CallToolResult, docListOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docListOutput{}, toolFailure(ctx, err)
	}

	projectID, err := t.optionalProject(ctx, workspace.ID, input.Project)
	if err != nil {
		return nil, docListOutput{}, toolFailure(ctx, err)
	}

	withinID, err := optionalDoc(input.Under)
	if err != nil {
		return nil, docListOutput{}, toolFailure(ctx, err)
	}

	list, err := t.docs.List(ctx, workspace.ID, service.ListDocsInput{
		Root:      projectID == nil && withinID == nil,
		ProjectID: projectID,
		WithinID:  withinID,
		Cursor:    input.Cursor,
		Limit:     input.Limit,
	})
	if err != nil {
		return nil, docListOutput{}, toolFailure(ctx, err)
	}

	output := docListOutput{Docs: make([]docSummaryDTO, 0, len(list.Docs)), NextCursor: list.NextCursor}

	for _, summary := range list.Docs {
		output.Docs = append(output.Docs, docSummaryDTOFrom(summary))
	}

	return nil, output, nil
}

func (t *toolset) getDoc(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getDocInput,
) (*mcp.CallToolResult, linkedDocOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, linkedDocOutput{}, toolFailure(ctx, err)
	}

	docID, err := docReference(input.Doc)
	if err != nil {
		return nil, linkedDocOutput{}, toolFailure(ctx, err)
	}

	doc, err := t.docs.Get(ctx, workspace.ID, docID)
	if err != nil {
		return nil, linkedDocOutput{}, toolFailure(ctx, err)
	}

	links, err := t.docLinks.List(ctx, workspace.ID, docID)
	if err != nil {
		return nil, linkedDocOutput{}, toolFailure(ctx, err)
	}

	output := linkedDocOutput{Doc: docDTOFrom(doc), Links: make([]docLinkDTO, 0, len(links))}

	for _, link := range links {
		output.Links = append(output.Links, docLinkDTOFrom(link))
	}

	return nil, output, nil
}

func (t *toolset) linkTarget(
	ctx context.Context,
	workspaceID uuid.UUID,
	input linkDocInput,
) (service.LinkDocInput, error) {
	var target service.LinkDocInput

	if strings.TrimSpace(input.Issue) != "" {
		issue, err := t.resolveIssue(ctx, workspaceID, input.Issue)
		if err != nil {
			return service.LinkDocInput{}, err
		}

		target.IssueID = issue.ID
	}

	if strings.TrimSpace(input.Project) != "" {
		project, err := t.resolveProject(ctx, workspaceID, input.Project)
		if err != nil {
			return service.LinkDocInput{}, err
		}

		target.ProjectID = project.Project.ID
	}

	if _, err := entity.DocLinkTargetOf(target.IssueID, target.ProjectID); err != nil {
		return service.LinkDocInput{}, refusal("invalid_input", "name an issue or a project to link, not both")
	}

	return target, nil
}

func (t *toolset) linkDoc(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input linkDocInput,
) (*mcp.CallToolResult, docLinkOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	docID, err := docReference(input.Doc)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	target, err := t.linkTarget(ctx, workspace.ID, input)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	link, err := t.docLinks.Link(ctx, workspace.ID, docID, target)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	return nil, docLinkOutput{Link: docLinkDTOFrom(link)}, nil
}

func (t *toolset) unlinkDoc(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input linkDocInput,
) (*mcp.CallToolResult, docLinkOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	docID, err := docReference(input.Doc)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	target, err := t.linkTarget(ctx, workspace.ID, input)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	links, err := t.docLinks.List(ctx, workspace.ID, docID)
	if err != nil {
		return nil, docLinkOutput{}, toolFailure(ctx, err)
	}

	for _, link := range links {
		if link.Issue.ID != target.IssueID || link.Project.ID != target.ProjectID {
			continue
		}

		if err := t.docLinks.Unlink(ctx, workspace.ID, docID, link.ID); err != nil {
			return nil, docLinkOutput{}, toolFailure(ctx, err)
		}

		return nil, docLinkOutput{Link: docLinkDTOFrom(link)}, nil
	}

	return nil, docLinkOutput{}, toolFailure(ctx, entity.ErrDocLinkNotFound)
}

func (t *toolset) createDoc(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createDocInput,
) (*mcp.CallToolResult, docOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	audience, err := t.audienceFor(ctx, workspace.ID, input.Visibility, input.Project)
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	parentID, err := optionalDoc(input.Parent)
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	doc, err := t.docs.Create(ctx, workspace.ID, service.CreateDocInput{
		Title:    input.Title,
		Markdown: input.Markdown,
		ParentID: parentID,
		Audience: audience,
	})
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	return nil, docOutput{Doc: docDTOFrom(doc)}, nil
}

func (t *toolset) editDoc(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input editDocInput,
) (*mcp.CallToolResult, docOutput, error) {
	if input.Title == nil && input.Markdown == nil {
		return nil, docOutput{}, toolFailure(ctx, refusal(
			"invalid_input", "name a title, markdown, or both; an edit that changes nothing writes nothing",
		))
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	docID, err := docReference(input.Doc)
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	doc, err := t.docs.Update(ctx, workspace.ID, docID, service.UpdateDocInput{
		ExpectedVersion: input.ExpectedVersion,
		Title:           input.Title,
		Markdown:        input.Markdown,
		Section:         input.Section,
	})
	if err != nil {
		return nil, docOutput{}, toolFailure(ctx, err)
	}

	return nil, docOutput{Doc: docDTOFrom(doc)}, nil
}

func (t *toolset) optionalProject(ctx context.Context, workspaceID uuid.UUID, ref string) (*uuid.UUID, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, nil
	}

	project, err := t.resolveProject(ctx, workspaceID, ref)
	if err != nil {
		return nil, err
	}

	return &project.Project.ID, nil
}

func (t *toolset) audienceFor(
	ctx context.Context,
	workspaceID uuid.UUID,
	visibility, project string,
) (entity.KnowledgeAudience, error) {
	projectID, err := t.optionalProject(ctx, workspaceID, project)
	if err != nil {
		return entity.KnowledgeAudience{}, err
	}

	chosen := entity.KnowledgeVisibility(strings.ToLower(strings.TrimSpace(visibility)))

	switch {
	case chosen == "" && projectID != nil, chosen == entity.KnowledgeVisibilityProject && projectID != nil:
		return entity.ProjectAudience(*projectID), nil
	case chosen == entity.KnowledgeVisibilityProject:
		return entity.KnowledgeAudience{}, refusal("invalid_input", "project visibility needs a project")
	case projectID != nil:
		return entity.KnowledgeAudience{}, refusal(
			"invalid_input", "a project is named only for project visibility; drop it or choose project",
		)
	case chosen == "", chosen == entity.KnowledgeVisibilityWorkspace:
		return entity.WorkspaceAudience(), nil
	case chosen == entity.KnowledgeVisibilityPersonal:
		return entity.PersonalAudience(), nil
	default:
		return entity.KnowledgeAudience{}, refusal(
			"invalid_input", "visibility is workspace, project, or personal",
		)
	}
}

func docReference(ref string) (uuid.UUID, error) {
	docID, err := uuid.Parse(strings.TrimSpace(ref))
	if err != nil {
		return uuid.Nil, entity.ErrDocNotFound
	}

	return docID, nil
}

func optionalDoc(ref string) (*uuid.UUID, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, nil
	}

	docID, err := docReference(ref)
	if err != nil {
		return nil, err
	}

	return &docID, nil
}
