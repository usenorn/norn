import { openCycles, phaseLabel, type Cycle } from "$lib/cycles/cycles";
import { selectable, type Label } from "$lib/labels/labels";
import type { WorkflowState } from "$lib/team/states";
import { assignees, type Membership } from "$lib/workspace/members";
import type { BulkChange } from "./apply";
import type { StepKind, StepOption } from "./model";

export const backlogOption = "backlog";

export type StepSources = {
	teamId: string | null;
	members: Membership[];
	states: WorkflowState[];
	labels: Label[];
	cycles: Cycle[];
};

export function stepOptions(kind: StepKind, sources: StepSources): StepOption[] {
	switch (kind) {
		case "assign":
			return assignees(sources.members).map((member) => {
				const name = member.displayName ?? member.email ?? "";

				return {
					value: member.accountId,
					label: name,
					hint: member.email,
					person: { accountId: member.accountId, name },
				};
			});
		case "status":
			return sources.states
				.filter((state) => state.teamId === sources.teamId)
				.sort((a, b) => a.position - b.position)
				.map((state) => ({ value: state.id, label: state.name, category: state.category }));
		case "cycle":
			return [
				...openCycles(sources.cycles, sources.teamId).map((cycle) => ({
					value: cycle.id,
					label: cycle.name,
					hint: phaseLabel(cycle.phase),
				})),
				{ value: backlogOption, label: "Backlog", hint: "no cycle" },
			];
		default:
			return selectable(sources.labels, sources.teamId).map((label) => ({
				value: label.id,
				label: label.name,
				dot: label.color,
			}));
	}
}

export function changeFor(kind: StepKind, value: string): BulkChange {
	switch (kind) {
		case "assign":
			return { assigneeId: value };
		case "status":
			return { stateId: value };
		case "cycle":
			return value === backlogOption ? { clearCycle: true } : { cycleId: value };
		default:
			return { addLabelId: value };
	}
}

export function appliedLine(kind: StepKind, subject: string, option: string): string {
	switch (kind) {
		case "assign":
			return `Assigned ${subject} to ${option}`;
		case "label":
			return `Added ${option} to ${subject}`;
		default:
			return `Moved ${subject} to ${option}`;
	}
}

export function queuedLine(subject: string): string {
	return `Changing ${subject} in the background`;
}

export function failedTitle(kind: StepKind, subject: string): string {
	switch (kind) {
		case "assign":
			return `Couldn’t assign ${subject}`;
		case "status":
			return `Couldn’t change the status of ${subject}`;
		case "cycle":
			return `Couldn’t move ${subject} to a cycle`;
		default:
			return `Couldn’t add a label to ${subject}`;
	}
}
