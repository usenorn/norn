import type { components } from "$lib/api/dashboard.gen";

export type Issue = components["schemas"]["Issue"];
export type IssuePriority = components["schemas"]["IssuePriority"];
export type IssueStatus = components["schemas"]["IssueStatus"];
export type IssueActivity = components["schemas"]["IssueActivity"];

export type IssueFailure =
	| { kind: "stale"; fields: string[] }
	| { kind: "labels_out_of_scope"; labels: string[] }
	| { kind: "already_on_team" }
	| { kind: "destination_incapable" }
	| { kind: "status_transition" }
	| { kind: "invalid"; fields: string[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const priorities: { value: IssuePriority; label: string }[] = [
	{ value: "urgent", label: "Urgent" },
	{ value: "high", label: "High" },
	{ value: "medium", label: "Medium" },
	{ value: "low", label: "Low" },
	{ value: "none", label: "No priority" },
];

export function priorityLabel(priority: IssuePriority): string {
	return priorities.find((entry) => entry.value === priority)?.label ?? "No priority";
}

export function statusLabel(status: IssueStatus): string {
	switch (status) {
		case "archived":
			return "Archived";
		case "pending_deletion":
			return "Deleted";
		default:
			return "Active";
	}
}

const fieldNames: Record<string, string> = {
	title: "the title",
	state: "the state",
	team: "the team",
	labels: "the labels",
	description: "the description",
	priority: "the priority",
	assignee: "the assignee",
	estimate: "the estimate",
	dueOn: "the due date",
	status: "whether it is archived",
};

export function nameFields(fields: string[]): string {
	const named = fields.map((field) => fieldNames[field] ?? field);

	if (named.length === 0) return "something";
	if (named.length === 1) return named[0];

	return `${named.slice(0, -1).join(", ")} and ${named[named.length - 1]}`;
}

export function issueFailureMessage(failure: IssueFailure): string {
	switch (failure.kind) {
		case "stale":
			return `Someone else changed ${nameFields(failure.fields)} while you were editing. Your copy has been refreshed with theirs — check it and try again.`;
		case "labels_out_of_scope":
			return `${failure.labels.join(", ")} ${failure.labels.length === 1 ? "belongs" : "belong"} to the team this issue is leaving. Moving it will take ${failure.labels.length === 1 ? "that label" : "those labels"} off.`;
		case "already_on_team":
			return "This issue is already on that team.";
		case "destination_incapable":
			return "That team has no state matching the one this issue is in.";
		case "status_transition":
			return "This issue is not in a state that allows that.";
		case "invalid":
			return `Check ${nameFields(failure.fields)}.`;
		case "forbidden":
			return "You do not have permission to change this issue.";
		default:
			return "Something went wrong and nothing changed. Wait a moment and try again.";
	}
}

export function readIssueFailure(error: unknown): IssueFailure {
	if (!error || typeof error !== "object") return { kind: "unavailable" };

	const problem = error as {
		code?: string;
		conflicts?: string[];
		labels?: { name: string }[];
		errors?: { field?: string }[];
		status?: number;
	};

	switch (problem.code) {
		case "issue_stale":
			return { kind: "stale", fields: problem.conflicts ?? [] };
		case "issue_labels_out_of_scope":
			return { kind: "labels_out_of_scope", labels: (problem.labels ?? []).map((l) => l.name) };
		case "issue_already_on_team":
			return { kind: "already_on_team" };
		case "issue_destination_incapable":
			return { kind: "destination_incapable" };
		case "issue_status_transition":
			return { kind: "status_transition" };
	}

	if (problem.errors) {
		return { kind: "invalid", fields: problem.errors.map((entry) => entry.field ?? "").filter(Boolean) };
	}

	if (problem.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

export function activityLine(entry: IssueActivity): string {
	switch (entry.kind) {
		case "created":
			return `Raised in ${entry.toState}`;
		case "state_changed":
			return `${entry.fromState} → ${entry.toState}`;
		case "team_moved":
			return `Moved from ${entry.fromValue} to ${entry.toValue}`;
		case "archived":
			return "Archived";
		case "unarchived":
			return "Taken out of the archive";
		case "deleted":
			return "Deleted";
		case "restored":
			return "Restored";
		case "property_changed":
			return propertyLine(entry);
		default:
			return "Changed";
	}
}

function propertyLine(entry: IssueActivity): string {
	const field = fieldNames[entry.field ?? ""] ?? entry.field ?? "a property";

	if (entry.fromValue && entry.toValue) return `Changed ${field} from ${entry.fromValue} to ${entry.toValue}`;
	if (entry.toValue) return `Set ${field} to ${entry.toValue}`;
	if (entry.fromValue) return `Cleared ${field}, which was ${entry.fromValue}`;

	return `Changed ${field}`;
}
