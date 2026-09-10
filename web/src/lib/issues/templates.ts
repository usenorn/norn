import { api } from "$lib/api";
import { documentEmpty, documentText, type Document } from "$lib/editor/document";
import type { components } from "$lib/api/dashboard.gen";

export type IssueTemplate = components["schemas"]["IssueTemplate"];
export type TemplateField = components["schemas"]["TemplateField"];

export type TemplateList =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; templates: IssueTemplate[] }
	| { kind: "unavailable" };

export type TemplateFailure =
	| { kind: "name_used" }
	| { kind: "too_many" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const templateFields: { field: TemplateField; label: string }[] = [
	{ field: "assignee", label: "Assignee" },
	{ field: "project", label: "Project" },
	{ field: "cycle", label: "Cycle" },
	{ field: "estimate", label: "Estimate" },
	{ field: "dueOn", label: "Due date" },
	{ field: "labels", label: "Labels" },
	{ field: "priority", label: "Priority" },
];

export function templatesFor(templates: IssueTemplate[] | undefined): TemplateList {
	if (!templates) return { kind: "unavailable" };

	return templates.length === 0 ? { kind: "empty" } : { kind: "ready", templates };
}

export function templatesOf(list: TemplateList): IssueTemplate[] {
	return list.kind === "ready" ? list.templates : [];
}

export function templateFailureMessage(failure: TemplateFailure): string {
	switch (failure.kind) {
		case "name_used":
			return "A template with that name is already kept here. Choose another name.";
		case "too_many":
			return "This team keeps as many templates as it can. Remove one before adding another.";
		case "forbidden":
			return "You may not change this team's templates.";
		default:
			return "We could not reach the server just now. Nothing was changed.";
	}
}

export function readTemplateFailure(error: unknown): TemplateFailure {
	const problem = error as { status?: number; detail?: string } | undefined;

	if (problem?.status === 403) return { kind: "forbidden" };

	if (problem?.detail?.includes("already exists")) return { kind: "name_used" };

	if (problem?.detail?.includes("too many")) return { kind: "too_many" };

	return { kind: "unavailable" };
}

export type TemplateDraft = {
	templateId?: string;
	teamId?: string;
	name: string;
	description: string;
	title: string;
	body: Document;
	requiredFields: TemplateField[];
	priority?: IssueTemplate["priority"];
	estimate?: number;
};

export async function saveTemplate(
	workspaceId: string,
	draft: TemplateDraft
): Promise<{ template?: IssueTemplate; failure?: TemplateFailure }> {
	try {
		const { data, error } = await api.PUT("/workspaces/{workspaceId}/issue-templates", {
			params: { path: { workspaceId } },
			body: {
				templateId: draft.templateId,
				teamId: draft.teamId,
				name: draft.name,
				description: draft.description || undefined,
				title: draft.title || undefined,
				body: documentText(draft.body) || undefined,
				bodyDoc: documentEmpty(draft.body) ? undefined : draft.body,
				requiredFields: draft.requiredFields,
				priority: draft.priority,
				estimate: draft.estimate && draft.estimate > 0 ? draft.estimate : undefined,
			},
		});

		if (error || !data) return { failure: readTemplateFailure(error) };

		return { template: data };
	} catch {
		return { failure: { kind: "unavailable" } };
	}
}

export async function readTemplates(
	workspaceId: string,
	teamId?: string
): Promise<IssueTemplate[] | undefined> {
	try {
		const { data, error } = await api.GET("/workspaces/{workspaceId}/issue-templates", {
			params: { path: { workspaceId }, query: teamId ? { teamId } : {} },
		});

		return error || !data ? undefined : data.templates;
	} catch {
		return undefined;
	}
}

export async function removeTemplate(workspaceId: string, templateId: string): Promise<boolean> {
	try {
		const { error } = await api.DELETE("/workspaces/{workspaceId}/issue-templates/{templateId}", {
			params: { path: { workspaceId, templateId } },
		});

		return !error;
	} catch {
		return false;
	}
}
