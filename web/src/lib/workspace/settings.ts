import { onDate } from "$lib/time";
import type { components, operations } from "$lib/api/dashboard.gen";

export type Workspace = components["schemas"]["Workspace"];
export type WeekDay = components["schemas"]["WeekDay"];

export type WorkspaceSettings =
	| { kind: "ready"; workspace: Workspace }
	| { kind: "pending_deletion"; workspace: Workspace; purgeAfter: string }
	| { kind: "saved"; workspace: Workspace; renamedFrom?: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SaveBar = { kind: "hidden" } | { kind: "unsaved" } | { kind: "saving" } | { kind: "conflict" };

export const weekDays: { value: WeekDay; label: string }[] = [
	{ value: "monday", label: "Monday" },
	{ value: "sunday", label: "Sunday" },
];

export const slugRedirectDays = 30;

export function saveBarOf(dirty: boolean, submitting: boolean, conflict: boolean): SaveBar {
	if (submitting) return { kind: "saving" };
	if (conflict) return { kind: "conflict" };
	if (dirty) return { kind: "unsaved" };

	return { kind: "hidden" };
}

export function saveBarLabel(bar: SaveBar): string {
	switch (bar.kind) {
		case "saving":
			return "Saving";
		case "conflict":
			return "Fix the identifier to save";
		default:
			return "Unsaved changes";
	}
}

export function redirectEnds(now: string, timezone: string): string {
	const ends = new Date(Date.parse(now) + slugRedirectDays * 24 * 60 * 60 * 1000).toISOString();

	return onDate(ends, timezone);
}

type UpdateResponses = operations["updateWorkspace"]["responses"];
type DeleteResponses = operations["deleteWorkspace"]["responses"];

export type UpdateProblem =
	UpdateResponses[401 | 403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export type DeleteProblem =
	DeleteResponses[401 | 403 | 404 | 409 | 500]["content"]["application/problem+json"];

export function settingsFor(workspace: Workspace): WorkspaceSettings {
	return workspace.status === "pending_deletion" && workspace.purgeAfter
		? { kind: "pending_deletion", workspace, purgeAfter: workspace.purgeAfter }
		: { kind: "ready", workspace };
}

export function nameMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter a workspace name.";
		case "too_long":
			return "Keep the name under 80 characters.";
		default:
			return "That name cannot be used.";
	}
}

export function slugMessage(code: string): string {
	switch (code) {
		case "taken":
			return "Taken by another workspace on this instance.";
		case "pinned":
			return "Single sign-on is set up on this address. Turn it off before changing the identifier.";
		case "required":
		case "too_short":
			return "Use at least 2 characters.";
		case "too_long":
			return "Keep the identifier under 40 characters.";
		default:
			return "Lowercase letters, numbers and dashes. It is the workspace address.";
	}
}

export const weekStartMessage = "Choose Monday or Sunday.";

export function timezoneMessage(code: string): string {
	switch (code) {
		case "required":
			return "Choose a timezone.";
		case "unknown_timezone":
			return "That is not a timezone this instance recognises.";
		default:
			return "That timezone cannot be used.";
	}
}

export function purgeDate(instant: string, timezone: string): string {
	return onDate(instant, timezone);
}

export function timezones(): string[] {
	const supported = Intl.supportedValuesOf?.("timeZone");

	return supported?.length ? supported : ["UTC"];
}
