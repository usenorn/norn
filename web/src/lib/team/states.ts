import type { components } from "$lib/api/dashboard.gen";

export type WorkflowState = components["schemas"]["WorkflowState"];
export type StateCategory = components["schemas"]["StateCategory"];

export const stateCategories: StateCategory[] = [
	"not_started",
	"active",
	"complete",
	"abandoned",
];

export const categoryLabels: Record<StateCategory, string> = {
	not_started: "Not started",
	active: "Active",
	complete: "Complete",
	abandoned: "Abandoned",
};

export const categoryHints: Record<StateCategory, string> = {
	not_started: "Filed, but nobody has picked it up.",
	active: "Someone is working on it.",
	complete: "Finished and counted as done.",
	abandoned: "Closed without being finished.",
};

export type StateList =
	| { kind: "loading" }
	| { kind: "ready"; states: WorkflowState[] }
	| { kind: "unavailable" };

export type StateFailure =
	| { kind: "name_taken" }
	| { kind: "is_default"; name: string }
	| { kind: "is_completion"; name: string }
	| { kind: "last_in_category"; category: StateCategory }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function statesFor(states: WorkflowState[] | undefined): StateList {
	if (!states) return { kind: "unavailable" };

	return { kind: "ready", states };
}

export function statesOf(list: StateList): WorkflowState[] {
	return list.kind === "ready" ? list.states : [];
}

export function stateFailureMessage(failure: StateFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "This team already has a state with that name.";
		case "is_default":
			return `${failure.name} is where new issues start. Make another state the default first.`;
		case "is_completion":
			return `${failure.name} is what counts as finished. Choose another completion state first.`;
		case "last_in_category":
			return `${categoryLabels[failure.category]} would be left with no states. Add another one first.`;
		case "forbidden":
			return "Only workspace admins can change a team's states.";
		default:
			return "Nothing changed. Wait a moment and try again.";
	}
}

export function conflictFailure(code: string, state: WorkflowState): StateFailure | null {
	switch (code) {
		case "state_name_taken":
			return { kind: "name_taken" };
		case "state_is_default":
			return { kind: "is_default", name: state.name };
		case "state_is_completion":
			return { kind: "is_completion", name: state.name };
		case "state_last_in_category":
			return { kind: "last_in_category", category: state.category };
		default:
			return null;
	}
}

export function byCategory(states: WorkflowState[], category: StateCategory): WorkflowState[] {
	return states.filter((state) => state.category === category);
}

export function isOnlyOneInCategory(states: WorkflowState[], state: WorkflowState): boolean {
	return byCategory(states, state.category).length === 1;
}

export function removalBlocker(
	states: WorkflowState[],
	state: WorkflowState
): StateFailure | null {
	if (state.isDefault) return { kind: "is_default", name: state.name };
	if (state.isCompletion) return { kind: "is_completion", name: state.name };
	if (isOnlyOneInCategory(states, state))
		return { kind: "last_in_category", category: state.category };

	return null;
}

export function reordered(states: WorkflowState[], id: string, delta: number): string[] {
	const ids = states.map((state) => state.id);
	const from = ids.indexOf(id);
	const to = from + delta;

	if (from < 0 || to < 0 || to >= ids.length) return ids;

	ids.splice(to, 0, ...ids.splice(from, 1));

	return ids;
}
