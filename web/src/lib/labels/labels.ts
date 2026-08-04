import type { components } from "$lib/api/dashboard.gen";

export type Label = components["schemas"]["Label"];
export type LabelGroup = components["schemas"]["LabelGroup"];
export type LabelColor = components["schemas"]["LabelColor"];

export const labelColors: LabelColor[] = [
	"neutral",
	"cyan",
	"blue",
	"violet",
	"orchid",
	"magenta",
];

export const colorLabels: Record<LabelColor, string> = {
	neutral: "Neutral",
	cyan: "Cyan",
	blue: "Blue",
	violet: "Violet",
	orchid: "Orchid",
	magenta: "Magenta",
};

export type LabelBoard =
	| { kind: "loading" }
	| { kind: "ready"; labels: Label[]; groups: LabelGroup[] }
	| { kind: "unavailable" };

export type LabelFailure =
	| { kind: "name_taken" }
	| { kind: "group_name_taken" }
	| { kind: "group_exclusive" }
	| { kind: "out_of_scope" }
	| { kind: "group_in_use" }
	| { kind: "usage_changed"; issues: number }
	| { kind: "merge_scope_narrows" }
	| { kind: "merge_group_mismatch" }
	| { kind: "stale"; conflicts: string[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function boardFor(
	labels: Label[] | undefined,
	groups: LabelGroup[] | undefined
): LabelBoard {
	if (!labels || !groups) return { kind: "unavailable" };

	return { kind: "ready", labels, groups };
}

export function labelsOf(board: LabelBoard): Label[] {
	return board.kind === "ready" ? board.labels : [];
}

export function groupsOf(board: LabelBoard): LabelGroup[] {
	return board.kind === "ready" ? board.groups : [];
}

export function labelFailureMessage(failure: LabelFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "A label with that name already exists in this scope.";
		case "group_name_taken":
			return "A group with that name already exists.";
		case "group_exclusive":
			return "An issue can carry only one label from a group.";
		case "out_of_scope":
			return "That label belongs to another team, so it cannot go on this issue.";
		case "group_in_use":
			return "That group still has labels in it.";
		case "usage_changed":
			return `This label is now on ${failure.issues} ${
				failure.issues === 1 ? "issue" : "issues"
			}. Check the new number before removing it.`;
		case "merge_scope_narrows":
			return "Merging into a team label would strand every issue outside that team. Merge the other way instead.";
		case "merge_group_mismatch":
			return "Only labels in the same group can be merged. Move one of them first.";
		case "stale":
			return `Someone else changed ${failure.conflicts.join(" and ")} while you were editing. Reload to see their change.`;
		case "forbidden":
			return "Only workspace admins can change labels.";
		default:
			return "Nothing changed. Wait a moment and try again.";
	}
}

export function conflictFailure(
	code: string,
	issues?: number,
	conflicts?: string[]
): LabelFailure | null {
	switch (code) {
		case "issue_stale":
			return { kind: "stale", conflicts: conflicts ?? [] };
		case "label_name_taken":
			return { kind: "name_taken" };
		case "label_group_name_taken":
			return { kind: "group_name_taken" };
		case "label_group_exclusive":
			return { kind: "group_exclusive" };
		case "label_out_of_scope":
			return { kind: "out_of_scope" };
		case "label_group_in_use":
			return { kind: "group_in_use" };
		case "label_usage_changed":
			return { kind: "usage_changed", issues: issues ?? 0 };
		case "label_merge_scope_narrows":
			return { kind: "merge_scope_narrows" };
		case "label_merge_group_mismatch":
			return { kind: "merge_group_mismatch" };
		default:
			return null;
	}
}

export type LabelSection = { group: LabelGroup | null; labels: Label[] };

export function sectioned(labels: Label[], groups: LabelGroup[]): LabelSection[] {
	const sections: LabelSection[] = groups.map((group) => ({
		group,
		labels: labels.filter((label) => label.groupId === group.id),
	}));

	sections.push({ group: null, labels: labels.filter((label) => !label.groupId) });

	return sections.filter((section) => section.labels.length > 0 || section.group !== null);
}

export function covers(target: Label, source: Label): boolean {
	if (target.workspaceId !== source.workspaceId) return false;

	return !target.teamId || target.teamId === source.teamId;
}

export function mergeTargets(source: Label, labels: Label[]): Label[] {
	return labels.filter(
		(candidate) =>
			candidate.id !== source.id &&
			candidate.groupId === source.groupId &&
			covers(candidate, source)
	);
}

export function appliesTo(label: Label, teamId: string): boolean {
	return !label.teamId || label.teamId === teamId;
}

export function selectable(labels: Label[], teamId: string): Label[] {
	return labels.filter((label) => appliesTo(label, teamId));
}

export function toggled(current: Label[], label: Label): string[] {
	if (current.some((applied) => applied.id === label.id)) {
		return current.filter((applied) => applied.id !== label.id).map((applied) => applied.id);
	}

	const kept = label.groupId
		? current.filter((applied) => applied.groupId !== label.groupId)
		: current;

	return [...kept.map((applied) => applied.id), label.id];
}
