import type { components } from "$lib/api/dashboard.gen";

export type DecisionRight = components["schemas"]["DecisionRight"];
export type ReviewQuestion = components["schemas"]["ReviewQuestion"];
export type ReviewRun = components["schemas"]["ReviewRun"];
export type WaitingDecisions = components["schemas"]["ReviewQueue"];

export type ReviewQueue =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "ready"; waiting: WaitingDecisions };

export const noReviewsLine =
	"Nothing is waiting. A decision appears here when a coding agent asks a question, has a plan for somebody to approve, or has changes for somebody to review.";

export function planningQuestions(waiting: WaitingDecisions): ReviewQuestion[] {
	return waiting.questions.filter((item) => item.question.stage === "planning");
}

export function implementationQuestions(waiting: WaitingDecisions): ReviewQuestion[] {
	return waiting.questions.filter((item) => item.question.stage !== "planning");
}

export function waitingTotal(waiting: WaitingDecisions): number {
	return waiting.questions.length + waiting.plans.length + waiting.changes.length;
}

export function waitingCount(waiting: WaitingDecisions): string {
	const total = waitingTotal(waiting);

	return total === 1 ? "1 decision is waiting" : `${total} decisions are waiting`;
}

export function waitingOnLine(right: DecisionRight): string {
	return right.decider
		? `Waiting on ${right.decider}. Only they or a workspace admin can decide this.`
		: "Only a workspace admin can decide this.";
}

const waitingSubjects = { plan: "The plan is", changes: "The changes are" } as const;

export function waitingTitle(what: keyof typeof waitingSubjects, right: DecisionRight): string {
	const subject = waitingSubjects[what];

	if (right.canDecide) return `${subject} waiting on you`;

	return `${subject} waiting on ${right.decider ?? "an admin"}`;
}
