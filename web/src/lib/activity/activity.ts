import type { components } from "$lib/api/dashboard.gen";

export type ActivityEvent = components["schemas"]["ActivityEvent"];
export type ActivityChange = components["schemas"]["ActivityChange"];
export type ActivityActorKind = components["schemas"]["ActivityActorKind"];

export type ActivityFeed =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; events: ActivityEvent[]; nextCursor?: string }
	| { kind: "unavailable" };

// Keyed by the link's kind, which is what source control puts in the change's field.
const codeKindNames: Record<string, string> = {
	branch: "branch",
	commit: "commit",
	change: "pull request",
};

const relationHeadings: Record<string, string> = {
	blocks: "blocks",
	blocked_by: "blocked by",
	duplicates: "duplicates",
	duplicated_by: "duplicated by",
	relates_to: "related to",
};

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
	parent: "the parent",
	children: "the children",
	project: "the project",
	cycle: "the cycle",
	name: "the name",
	lead: "the lead",
	targetOn: "the target date",
};

const triageDecisions: Record<string, string> = {
	accepted: "Accepted into the backlog",
	declined: "Declined",
	merged: "Merged as a duplicate of",
	reassigned: "Sent to",
};

export function changeLine(change: ActivityChange): string {
	switch (change.kind) {
		case "created":
			return change.toState ? `Raised in ${change.toState}` : "Raised";
		case "state_changed":
			return `${change.fromState ?? "somewhere"} → ${change.toState ?? "somewhere"}`;
		case "team_moved":
			return `Moved from ${change.fromValue} to ${change.toValue}`;
		case "child_added":
			return `${change.toValue} was filed under this`;
		case "child_removed":
			return `${change.fromValue} is no longer filed under this`;
		case "relation_added":
			return `Now ${relationHeadings[change.field ?? "relates_to"] ?? "related to"} ${change.toValue}`;
		case "relation_removed":
			return `No longer linked to ${change.fromValue}`;
		case "archived":
			return "Archived";
		case "unarchived":
			return "Taken out of the archive";
		case "deleted":
			return "Deleted";
		case "restored":
			return "Restored";
		case "commented":
			return "Commented";
		case "comment_deleted":
			return "Deleted a comment";
		case "member_added":
			return `${change.toValue} joined`;
		case "member_removed":
			return `${change.fromValue} left`;
		case "attachment_added":
			return `Attached ${change.toValue}`;
		case "attachment_removed":
			return `Removed ${change.toValue || change.fromValue}`;
		case "code_linked":
			return `Linked ${codeKindNames[change.field ?? ""] ?? "a change"} ${change.toValue}`;
		case "code_unlinked":
			return `Unlinked ${codeKindNames[change.field ?? ""] ?? "a change"} ${change.toValue}`;
		case "triaged":
			return triageLine(change);
		case "property_changed":
			return propertyLine(change);
		default:
			return "Changed something";
	}
}

function triageLine(change: ActivityChange): string {
	const decision = triageDecisions[change.field ?? ""] ?? "Triaged";

	return change.toValue ? `${decision} ${change.toValue}` : decision;
}

function propertyLine(change: ActivityChange): string {
	const field = fieldNames[change.field ?? ""] ?? change.field ?? "a property";

	if (change.field === "description") return "Edited the description";
	if (change.fromValue && change.toValue) {
		return `Changed ${field} from ${change.fromValue} to ${change.toValue}`;
	}
	if (change.toValue) return `Set ${field} to ${change.toValue}`;
	if (change.fromValue) return `Cleared ${field}, which was ${change.fromValue}`;

	return `Changed ${field}`;
}

export function actorLabel(event: ActivityEvent): string {
	if (event.actorKind === "system") return "Norn";

	const person = event.actorName || "Someone who has left";

	return event.actorTokenName ? `${person} via ${event.actorTokenName}` : person;
}

export const actorKindLabels: Record<ActivityActorKind, string> = {
	user: "A person made this change",
	token: "An API token made this change",
	agent: "An agent made this change",
	system: "Norn made this change",
};

export function readable(event: ActivityEvent): boolean {
	return event.changes.some(
		(change) => change.kind !== "commented" && change.kind !== "comment_deleted"
	);
}
