import { api } from "$lib/api";
import { documentEmpty, documentText, type Document } from "$lib/editor/document";
import type { components } from "$lib/api/dashboard.gen";

export type IssueDraft = components["schemas"]["IssueDraft"];

export type DraftFields = {
	draftId?: string;
	teamId?: string;
	title: string;
	description: Document;
	stateId?: string;
	projectId?: string;
	cycleId?: string;
	assigneeId?: string;
	parentIssueId?: string;
	labelIds?: string[];
	attachmentIds?: string[];
	priority?: IssueDraft["priority"];
	estimate?: number;
	dueOn?: string;
};

/**
 * A draft is worth keeping only when something was actually written. Keeping an empty one turns
 * opening the dialog and changing your mind into a row somebody has to clear later.
 */
export function worthKeeping(fields: DraftFields): boolean {
	return fields.title.trim() !== "" || !documentEmpty(fields.description);
}

export async function keepDraft(
	workspaceId: string,
	fields: DraftFields
): Promise<IssueDraft | null> {
	try {
		const { data, error } = await api.PUT("/workspaces/{workspaceId}/issue-drafts", {
			params: { path: { workspaceId } },
			body: {
				draftId: fields.draftId,
				teamId: fields.teamId || undefined,
				title: fields.title,
				description: documentText(fields.description),
				descriptionDoc: documentEmpty(fields.description) ? undefined : fields.description,
				stateId: fields.stateId || undefined,
				projectId: fields.projectId || undefined,
				cycleId: fields.cycleId || undefined,
				assigneeId: fields.assigneeId || undefined,
				parentIssueId: fields.parentIssueId || undefined,
				labelIds: fields.labelIds?.length ? fields.labelIds : undefined,
				attachmentIds: fields.attachmentIds?.length ? fields.attachmentIds : undefined,
				priority: fields.priority,
				estimate: fields.estimate && fields.estimate > 0 ? fields.estimate : undefined,
				dueOn: fields.dueOn || undefined,
			},
		});

		return error ? null : (data ?? null);
	} catch {
		return null;
	}
}

export async function readDrafts(workspaceId: string): Promise<IssueDraft[]> {
	try {
		const { data, error } = await api.GET("/workspaces/{workspaceId}/issue-drafts", {
			params: { path: { workspaceId } },
		});

		return error || !data ? [] : data.drafts;
	} catch {
		return [];
	}
}

export async function dropDraft(workspaceId: string, draftId: string): Promise<boolean> {
	try {
		const { error } = await api.DELETE("/workspaces/{workspaceId}/issue-drafts/{draftId}", {
			params: { path: { workspaceId, draftId } },
		});

		return !error;
	} catch {
		return false;
	}
}

export function draftLabel(draft: IssueDraft): string {
	const title = draft.title.trim();

	if (title !== "") return title;

	const words = draft.description.trim().split(/\s+/).slice(0, 8).join(" ");

	return words === "" ? "An untitled draft" : words;
}
