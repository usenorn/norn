import { onFullDateAndTime } from "$lib/time";
import type { Execution, IssueQuestion } from "./executions";
import { latestPlan, planStanding, revisionLabel, type ExecutionPlan } from "./plans";

export type ReviewLinks = { issue: string; run: string };

export type ReviewInput = {
	execution: Execution;
	plan: ExecutionPlan;
	plans: ExecutionPlan[];
	questions: IssueQuestion[];
	timezone: string;
	links: ReviewLinks;
};

export const copiedPlanLine = "Copied the plan for review";

export const questionsMissingLine =
	"The run's questions did not load, so this copy would be incomplete. Reload the page and try again.";

function quoted(text: string): string {
	return text
		.split("\n")
		.map((line) => `> ${line}`)
		.join("\n");
}

function standingLine(plan: ExecutionPlan, latest: ExecutionPlan | undefined, timezone: string) {
	const standing = planStanding(plan, latest);

	switch (standing.kind) {
		case "approved":
			return `Approved by ${standing.by} on ${onFullDateAndTime(standing.at, timezone)}`;
		case "revision_requested":
			return `${standing.by} asked for changes on ${onFullDateAndTime(standing.at, timezone)}`;
		case "waiting":
			return "Waiting for approval";
		case "superseded":
			return "Replaced by a newer revision";
	}
}

function feedbackOf(plan: ExecutionPlan, latest: ExecutionPlan | undefined): string[] {
	const standing = planStanding(plan, latest);

	return standing.kind === "revision_requested" && standing.feedback
		? ["", quoted(standing.feedback)]
		: [];
}

function questionStanding(question: IssueQuestion, timezone: string): string {
	const by = question.answeredByName ?? question.settledByName ?? "somebody";

	switch (question.state) {
		case "answered":
			return `Answered by ${by}${question.answeredAt ? ` on ${onFullDateAndTime(question.answeredAt, timezone)}` : ""}: ${question.answer ?? question.standing}`;
		case "dismissed":
			return `Dismissed by ${question.settledByName ?? "somebody"}${question.settledAt ? ` on ${onFullDateAndTime(question.settledAt, timezone)}` : ""}`;
		case "expired":
			return `Expired with no answer; the default stands: ${question.default}`;
		default:
			return question.expired
				? `Expired with no answer; the default stands: ${question.default}`
				: "Open, nobody has answered yet";
	}
}

function questionBlock(question: IssueQuestion, index: number, timezone: string): string[] {
	const stage = question.stage ? ` [${question.stage}]` : "";
	const lines = [
		`### ${index + 1}.${stage} ${question.question}`,
		"",
		`- Asked by: ${question.askedByName ?? "the coding agent"} on ${onFullDateAndTime(question.createdAt, timezone)}`,
		`- Kind: ${question.kind}${question.blocking ? ", blocking the run" : ""}`,
	];

	if (question.options && question.options.length > 0) {
		lines.push("- Options:", ...question.options.map((option) => `  - ${option}`));
		lines.push(
			question.allowFreeText
				? "- Free text: allowed alongside the options"
				: "- Free text: not allowed, only one of the options"
		);
	} else {
		lines.push("- Options: none, an open question");
	}

	lines.push(`- Default if nobody answers: ${question.default}`);
	lines.push(`- Deadline: ${onFullDateAndTime(question.deadline, timezone)}`);
	lines.push(`- Standing: ${questionStanding(question, timezone)}`);

	return lines;
}

export function reviewText(input: ReviewInput): string {
	const { execution, plan, plans, questions, timezone, links } = input;
	const latest = latestPlan(plans);
	const title = execution.issueTitle
		? `${execution.issueReference} — ${execution.issueTitle}`
		: execution.issueReference;
	const run = execution.agentName
		? `${execution.reference}, run by ${execution.agentName}`
		: execution.reference;

	const lines: string[] = [
		`# ${title}`,
		"",
		`- Issue: ${execution.issueReference} — ${links.issue}`,
		`- Run: ${run} — ${links.run}`,
		`- Times in ${timezone}`,
		"",
		`## Plan — ${revisionLabel(plan)} of ${plans.length}`,
		"",
		`- Proposed: ${onFullDateAndTime(plan.proposedAt, timezone)}`,
		`- Status: ${standingLine(plan, latest, timezone)}`,
		...feedbackOf(plan, latest),
	];

	if (latest && latest.revision !== plan.revision) {
		lines.push(
			"",
			`> A newer revision exists: ${revisionLabel(latest)}, proposed ${onFullDateAndTime(latest.proposedAt, timezone)}. This copy is of ${revisionLabel(plan)}.`
		);
	}

	lines.push("", "### Plan text", "", plan.body);

	if (plans.length > 1) {
		lines.push("", "## Revision history", "");

		for (const held of [...plans].sort((a, b) => a.revision - b.revision)) {
			const marker = held.revision === plan.revision ? " (this copy)" : "";

			lines.push(
				`- ${revisionLabel(held)}${marker}: proposed ${onFullDateAndTime(held.proposedAt, timezone)}; ${standingLine(held, latest, timezone)}`
			);

			const standing = planStanding(held, latest);

			if (standing.kind === "revision_requested" && standing.feedback) {
				lines.push(`  ${revisionLabel(held)} change request:`, ...quoted(standing.feedback).split("\n").map((line) => `  ${line}`));
			}
		}
	}

	if (questions.length === 0) {
		lines.push("", "## Questions", "", "No questions were asked during this run.");
	} else {
		lines.push("", `## Questions (${questions.length})`);

		questions.forEach((question, index) => {
			lines.push("", ...questionBlock(question, index, timezone));
		});
	}

	return lines.join("\n") + "\n";
}
