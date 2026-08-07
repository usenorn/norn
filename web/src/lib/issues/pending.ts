import type { Issue } from "./issues";

export type PendingEdit = { issueId: string; fields: Partial<Issue> };

export function withEdit(held: PendingEdit[], issueId: string, fields: Partial<Issue>): PendingEdit[] {
	const merged = held.find((edit) => edit.issueId === issueId)?.fields ?? {};

	return [...held.filter((edit) => edit.issueId !== issueId), { issueId, fields: { ...merged, ...fields } }];
}

export function without(held: PendingEdit[], issueId: string): PendingEdit[] {
	return held.filter((edit) => edit.issueId !== issueId);
}

export function edited(issue: Issue, edits: PendingEdit[]): Issue {
	const held = edits.find((edit) => edit.issueId === issue.id);

	return held ? { ...issue, ...held.fields } : issue;
}
