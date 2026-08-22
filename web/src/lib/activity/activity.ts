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

export type ActivityKind = components["schemas"]["ActivityKind"];

const changeLines: Record<ActivityKind, (change: ActivityChange) => string> = {
	created: (change) => (change.toState ? `Raised in ${change.toState}` : "Raised"),
	state_changed: (change) =>
		`${change.fromState ?? "somewhere"} → ${change.toState ?? "somewhere"}`,
	team_moved: (change) => `Moved from ${change.fromValue} to ${change.toValue}`,
	child_added: (change) => `${change.toValue} was filed under this`,
	child_removed: (change) => `${change.fromValue} is no longer filed under this`,
	relation_added: (change) =>
		`Now ${relationHeadings[change.field ?? "relates_to"] ?? "related to"} ${change.toValue}`,
	relation_removed: (change) => `No longer linked to ${change.fromValue}`,
	archived: () => "Archived",
	unarchived: () => "Taken out of the archive",
	deleted: () => "Deleted",
	restored: () => "Restored",
	commented: () => "Commented",
	comment_deleted: () => "Deleted a comment",
	member_added: (change) => `${change.toValue} joined`,
	member_removed: (change) => `${change.fromValue} left`,
	attachment_added: (change) => `Attached ${change.toValue}`,
	attachment_removed: (change) => `Removed ${change.toValue || change.fromValue}`,
	code_linked: (change) =>
		`Linked ${codeKindNames[change.field ?? ""] ?? "a change"} ${change.toValue}`,
	code_unlinked: (change) =>
		`Unlinked ${codeKindNames[change.field ?? ""] ?? "a change"} ${change.toValue}`,
	delegated: (change) => `Handed this to ${change.toValue}`,
	recalled: (change) => `Took this back from ${change.fromValue}`,
	question_asked: (change) => `Asked: ${change.toValue}`,
	question_answered: (change) => `Answered “${change.fromValue}” with “${change.toValue}”`,
	triaged: triageLine,
	property_changed: propertyLine,
};

export function changeLine(change: ActivityChange): string {
	return changeLines[change.kind](change);
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

	if (!event.actorTokenName || event.actorTokenName === person) return person;

	return `${person} via ${event.actorTokenName}`;
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
