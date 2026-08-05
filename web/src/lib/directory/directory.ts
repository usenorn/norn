import type { components, operations } from "$lib/api/dashboard.gen";

export type DirectorySettings = components["schemas"]["DirectorySettings"];
export type DirectoryConnection = components["schemas"]["DirectoryConnection"];
export type DirectoryRun = components["schemas"]["DirectoryRun"];
export type DirectoryChange = components["schemas"]["DirectoryChange"];
export type DirectoryUnknownPolicy = components["schemas"]["DirectoryUnknownPolicy"];
export type DirectoryAbsentPolicy = components["schemas"]["DirectoryAbsentPolicy"];

type ConnectResponses = operations["connectWorkspaceDirectory"]["responses"];

export type DirectoryProblem =
	ConnectResponses[403 | 500]["content"]["application/problem+json"];

export type DirectoryView =
	| { kind: "loading" }
	| { kind: "unlicensed"; holder?: string }
	| { kind: "disconnected" }
	| { kind: "connected"; connection: DirectoryConnection; scimBaseUrl: string; runs: DirectoryRun[] }
	| { kind: "issued"; connection: DirectoryConnection; scimBaseUrl: string; runs: DirectoryRun[]; token: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type DirectoryFailure =
	| { kind: "unlicensed" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function failureMessage(failure: DirectoryFailure): string {
	switch (failure.kind) {
		case "unlicensed":
			return "Directory synchronization is not part of this instance's licence.";
		case "forbidden":
			return "You need to administer this workspace to change how it is provisioned.";
		default:
			return "Check your connection and try again.";
	}
}

export const unknownPolicies: { value: DirectoryUnknownPolicy; label: string; note: string }[] = [
	{
		value: "provision",
		label: "Create them",
		note: "Somebody the directory sends who is not here yet gets an account and a membership.",
	},
	{
		value: "ignore",
		label: "Ignore them",
		note: "Only people who already have an account here are synced.",
	},
];

export const absentPolicies: { value: DirectoryAbsentPolicy; label: string; note: string }[] = [
	{
		value: "deactivate",
		label: "Deactivate them",
		note: "Their access ends at once. Nothing they wrote is deleted, and their open work is listed in the log.",
	},
	{
		value: "flag",
		label: "Record it and wait",
		note: "The departure is logged and their access continues until somebody acts on it.",
	},
];

const changeLabels: Record<string, string> = {
	provisioned: "Created",
	adopted: "Taken over",
	updated: "Updated",
	deactivated: "Deactivated",
	reactivated: "Reactivated",
	role_changed: "Role changed",
	team_joined: "Added to team",
	team_left: "Removed from team",
	group_unmapped: "Skipped",
	refused: "Refused",
};

export function changeLabel(kind: string): string {
	return changeLabels[kind] ?? kind;
}

const operationLabels: Record<string, string> = {
	PutUser: "A person was sent",
	SetUserActive: "A person was activated or suspended",
	DeleteUser: "A person was removed",
	PutGroup: "A group was sent",
	EditGroupMembers: "Group membership changed",
	DeleteGroup: "A group was removed",
};

export function operationLabel(operation: string): string {
	return operationLabels[operation] ?? operation;
}

export function detailPairs(change: DirectoryChange): [string, string][] {
	return Object.entries(change.detail ?? {})
		.filter(([, value]) => value !== "")
		.map(([key, value]) => [key.replace(/_/g, " "), value]);
}
