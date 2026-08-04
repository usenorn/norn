import { onDate } from "$lib/time";
import type { components, operations } from "$lib/api/dashboard.gen";

export type Workspace = components["schemas"]["Workspace"];

export type WorkspaceSettings =
	| { kind: "ready"; workspace: Workspace }
	| { kind: "pending_deletion"; workspace: Workspace; purgeAfter: string }
	| { kind: "saved"; workspace: Workspace }
	| { kind: "unavailable"; workspace: Workspace };

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
