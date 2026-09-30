import type { components } from "$lib/api/dashboard.gen";
import type { Step } from "$lib/components/norn/step-list.svelte";
import { isSettled, type Execution, type IssueQuestion } from "./executions";
import { waitingOnLine, type DecisionRight } from "./reviews";

export type ExecutionPlan = components["schemas"]["ExecutionPlan"];
export type ExecutionStage = components["schemas"]["ExecutionStage"];

export type PlanStanding =
	| { kind: "approved"; by: string; at: string }
	| { kind: "revision_requested"; by: string; at: string; feedback: string }
	| { kind: "waiting" }
	| { kind: "superseded" };

export function latestPlan(plans: ExecutionPlan[]): ExecutionPlan | undefined {
	return plans.reduce<ExecutionPlan | undefined>(
		(latest, plan) => (!latest || plan.revision > latest.revision ? plan : latest),
		undefined
	);
}

export function planStanding(plan: ExecutionPlan, latest: ExecutionPlan | undefined): PlanStanding {
	if (plan.approvedAt) {
		return { kind: "approved", by: plan.approvedByName || "Somebody", at: plan.approvedAt };
	}

	if (plan.revisionRequestedAt) {
		return {
			kind: "revision_requested",
			by: plan.revisionRequestedByName || "Somebody",
			at: plan.revisionRequestedAt,
			feedback: plan.revisionFeedback ?? "",
		};
	}

	if (latest && plan.revision === latest.revision) return { kind: "waiting" };

	return { kind: "superseded" };
}

export type PlanDecision =
	| { kind: "open" }
	| { kind: "blocked"; reason: string }
	| { kind: "not_yours"; reason: string }
	| { kind: "closed" };

export function planDecision(
	execution: Execution,
	plan: ExecutionPlan,
	latest: ExecutionPlan | undefined,
	questions: IssueQuestion[],
	right: DecisionRight
): PlanDecision {
	if (execution.state !== "awaiting_plan_approval") return { kind: "closed" };
	if (!latest || plan.revision !== latest.revision) return { kind: "closed" };
	if (planStanding(plan, latest).kind !== "waiting") return { kind: "closed" };
	if (!right.canDecide) return { kind: "not_yours", reason: waitingOnLine(right) };

	const open = questions.filter(
		(question) => question.blocking && question.state === "asked" && !question.expired
	).length;

	if (open === 1) return { kind: "blocked", reason: "Answer the open question before approving." };
	if (open > 1) return { kind: "blocked", reason: `Answer the ${open} open questions before approving.` };

	return { kind: "open" };
}

export function noPlanLine(execution: Execution): string {
	if (isSettled(execution.state)) return "This run stopped before it proposed a plan.";

	if (execution.state === "waiting_for_input") {
		return "The coding agent stopped to ask something before it could finish the plan.";
	}

	return "The coding agent is reading the code and writing a plan. Nothing in the workspace changes until somebody approves it.";
}

export function revisionLabel(plan: ExecutionPlan): string {
	return `Revision ${plan.revision}`;
}

const lifecycle = ["Plan", "Approve the plan", "Build", "Review the changes", "Publish"];

function position(execution: Execution): number {
	switch (execution.stage) {
		case "planning":
			return execution.state === "awaiting_plan_approval" ? 1 : 0;
		case "implementation":
			return 2;
		case "review":
			return 3;
		case "publication":
			return execution.state === "completed" ? lifecycle.length : 4;
	}
}

export function lifecycleSteps(execution: Execution): Step[] {
	const reached = position(execution);
	const stopped = isSettled(execution.state) && execution.state !== "completed";

	return lifecycle.map((label, index) => ({
		label,
		state:
			index < reached ? "done" : index > reached ? "waiting" : stopped ? "stopped" : "active",
	}));
}

export const planFeedbackMaxLength = 4000;
