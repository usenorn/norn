import type { Grouping, Ordering } from "./display";
import type { Issue, IssuePriority } from "./issues";

export type DropTarget = { key: string; index: number };

export type IssueMove = {
	stateId?: string;
	priority?: IssuePriority;
	assigneeId?: string;
	projectId?: string;
	dueOn?: string;
	clear?: string[];
	afterIssueId?: string;
	beforeIssueId?: string;
};

export type PendingMove = {
	issueId: string;
	key: string;
	index: number;
	fields: Partial<Issue>;
};

export type Landing = { move: IssueMove; pending: PendingMove };

export function insertionIndex(column: Element, pointerY: number): number {
	const cards = [...column.querySelectorAll<HTMLElement>("[data-issue]")];

	for (const [index, card] of cards.entries()) {
		const box = card.getBoundingClientRect();

		if (pointerY < box.top + box.height / 2) return index;
	}

	return cards.length;
}

export function applyMoves(
	held: Map<string, Issue[]>,
	moves: PendingMove[],
	known: Map<string, Issue>
): Map<string, Issue[]> {
	if (moves.length === 0) return held;

	const arranged = new Map([...held].map(([key, issues]) => [key, [...issues]]));

	for (const move of moves) {
		const issue = known.get(move.issueId);
		if (!issue) continue;

		for (const [key, issues] of arranged) {
			arranged.set(
				key,
				issues.filter((candidate) => candidate.id !== move.issueId)
			);
		}

		const into = arranged.get(move.key) ?? [];
		const at = Math.min(move.index, into.length);

		into.splice(at, 0, { ...issue, ...move.fields });
		arranged.set(move.key, into);
	}

	return arranged;
}

function groupMove(grouping: Grouping, key: string): IssueMove {
	switch (grouping) {
		case "priority":
			return { priority: key as IssuePriority };
		case "assignee":
			return key ? { assigneeId: key } : { clear: ["assignee"] };
		case "project":
			return key ? { projectId: key } : { clear: ["project"] };
		case "none":
			return {};
		default:
			return { stateId: key };
	}
}

function groupFields(grouping: Grouping, key: string, held: Issue[]): Partial<Issue> {
	const sibling = held[0];

	switch (grouping) {
		case "priority":
			return { priority: key as IssuePriority };
		case "assignee":
			return { assigneeAccountId: key || undefined };
		case "project":
			return { projectId: key || undefined, projectName: sibling?.projectName };
		case "none":
			return {};
		default:
			return sibling ? { state: sibling.state } : {};
	}
}

function inherited(ordering: Ordering, above: Issue | undefined): Partial<Issue> {
	if (ordering === "priority") return { priority: above?.priority ?? "none" };
	if (ordering === "due") return { dueOn: above?.dueOn };

	return {};
}

export function landing(
	dragged: Issue,
	held: Issue[],
	target: DropTarget,
	grouping: Grouping,
	ordering: Ordering
): Landing {
	const without = held.filter((issue) => issue.id !== dragged.id);
	const at = held.findIndex((issue) => issue.id === dragged.id);
	const index = at >= 0 && at < target.index ? target.index - 1 : target.index;

	const above = without[index - 1];
	const below = without[index];
	const carried = inherited(ordering, above);

	const grouped = groupMove(grouping, target.key);
	const cleared = [...(grouped.clear ?? [])];

	if (ordering === "due" && !carried.dueOn) cleared.push("dueOn");

	const move: IssueMove = {
		...grouped,
		afterIssueId: above?.id,
		beforeIssueId: below?.id,
		...(carried.priority ? { priority: carried.priority } : {}),
		...(carried.dueOn ? { dueOn: carried.dueOn } : {}),
		...(cleared.length > 0 ? { clear: cleared } : {}),
	};

	return {
		move,
		pending: {
			issueId: dragged.id,
			key: target.key,
			index,
			fields: { ...groupFields(grouping, target.key, held), ...carried },
		},
	};
}

export function movedInto(move: IssueMove): boolean {
	return Object.values(move).some((value) => value !== undefined);
}

export function stayedPut(
	dragged: Issue,
	held: Issue[],
	target: DropTarget,
	grouping: Grouping
): boolean {
	if (keyOf(grouping, dragged) !== target.key) return false;

	const at = held.findIndex((issue) => issue.id === dragged.id);

	return at === target.index || at === target.index - 1;
}

export function keyOf(grouping: Grouping, issue: Issue): string {
	switch (grouping) {
		case "priority":
			return issue.priority;
		case "assignee":
			return issue.assigneeAccountId ?? "";
		case "project":
			return issue.projectId ?? "";
		case "none":
			return "all";
		default:
			return issue.state.id;
	}
}
