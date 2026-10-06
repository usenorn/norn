import { describe, expect, it } from "vitest";
import type { Execution, IssueQuestion } from "./executions";
import { reviewText, type ReviewInput } from "./plan-review";
import type { ExecutionPlan } from "./plans";

const timezone = "Asia/Yerevan";
const links = {
	issue: "https://app.norn.so/moov/issues/MOO-45",
	run: "https://app.norn.so/moov/executions/exec-45",
};

function execution(fields: Partial<Execution> = {}): Execution {
	return {
		id: "exec-45",
		reference: "MOO-45",
		issueReference: "MOO-45",
		issueTitle: "(Backoffice) Filters addition in some reports / sections",
		agentName: "ero-delegate-agent",
		state: "awaiting_plan_approval",
		stage: "planning",
		...fields,
	} as Execution;
}

function plan(fields: Partial<ExecutionPlan> & { revision: number }): ExecutionPlan {
	return {
		id: `plan-${fields.revision}`,
		executionId: "exec-45",
		body: `## Revision ${fields.revision} body`,
		proposedAt: "2026-10-05T19:20:00Z",
		...fields,
	};
}

function question(fields: Partial<IssueQuestion> = {}): IssueQuestion {
	return {
		id: "q-1",
		issueId: "issue-45",
		issueReference: "MOO-45",
		issueTitle: "Filters",
		executionId: "exec-45",
		stage: "planning",
		kind: "decision",
		state: "asked",
		blocking: true,
		options: ["Per report", "Globally"],
		allowFreeText: false,
		question: "Should the filters persist?",
		default: "Per report",
		deadline: "2026-10-06T19:20:00Z",
		answered: false,
		expired: false,
		standing: "Per report",
		askedByName: "ero-delegate-agent",
		actorKind: "agent",
		createdAt: "2026-10-05T18:10:00Z",
		...fields,
	} as IssueQuestion;
}

function input(fields: Partial<ReviewInput> = {}): ReviewInput {
	const only = plan({ revision: 1 });

	return {
		execution: execution(),
		plan: only,
		plans: [only],
		questions: [],
		timezone,
		links,
		...fields,
	};
}

describe("the text copied for review", () => {
	it("names the issue, the run and both links, and says which timezone the times are in", () => {
		const text = reviewText(input());

		expect(text).toContain("# MOO-45 — (Backoffice) Filters addition in some reports / sections");
		expect(text).toContain("- Issue: MOO-45 — https://app.norn.so/moov/issues/MOO-45");
		expect(text).toContain("- Run: MOO-45, run by ero-delegate-agent — https://app.norn.so/moov/executions/exec-45");
		expect(text).toContain("- Times in Asia/Yerevan");
	});

	it("carries a long plan body whole, with nothing trimmed or reflowed", () => {
		const body = Array.from({ length: 400 }, (_, line) => `## Section ${line}\n\nParagraph ${line} with **bold** and \`code\`.`).join("\n\n");
		const text = reviewText(input({ plan: plan({ revision: 1, body }), plans: [plan({ revision: 1, body })] }));

		expect(text).toContain("### Plan text\n\n" + body + "\n");
	});

	it("says there were no questions rather than leaving the section out", () => {
		expect(reviewText(input())).toContain("## Questions\n\nNo questions were asked during this run.");
	});

	it("lists every option, the default, the deadline and the answer of an answered question", () => {
		const answered = question({
			state: "answered",
			answered: true,
			answer: "Globally",
			standing: "Globally",
			answeredByName: "Ero",
			answeredAt: "2026-10-05T18:30:00Z",
		});
		const text = reviewText(input({ questions: [answered] }));

		expect(text).toContain("## Questions (1)");
		expect(text).toContain("### 1. [planning] Should the filters persist?");
		expect(text).toContain("- Options:\n  - Per report\n  - Globally");
		expect(text).toContain("- Free text: not allowed, only one of the options");
		expect(text).toContain("- Default if nobody answers: Per report");
		expect(text).toContain("- Deadline: 6 Oct 2026, 23:20");
		expect(text).toContain("- Standing: Answered by Ero on 5 Oct 2026, 22:30: Globally");
	});

	it("marks an unanswered question as open and an expired one as standing on its default", () => {
		const open = question({ id: "q-open" });
		const expired = question({ id: "q-late", expired: true, question: "Keep the old endpoint?" });
		const text = reviewText(input({ questions: [open, expired] }));

		expect(text).toContain("- Standing: Open, nobody has answered yet");
		expect(text).toContain("- Standing: Expired with no answer; the default stands: Per report");
		expect(text).toContain("- Kind: decision, blocking the run");
	});

	it("labels a change request with the revision it was made on and never claims a reply", () => {
		const first = plan({
			revision: 1,
			revisionFeedback: "Drop the old endpoint.\nNothing reads it.",
			revisionRequestedByName: "Rae Okafor",
			revisionRequestedAt: "2026-10-05T19:40:00Z",
		});
		const second = plan({ revision: 2, proposedAt: "2026-10-05T20:00:00Z" });
		const text = reviewText(input({ plan: second, plans: [first, second] }));

		expect(text).toContain("## Plan — Revision 2 of 2");
		expect(text).toContain("- Status: Waiting for approval");
		expect(text).toContain("## Revision history");
		expect(text).toContain(
			"- Revision 1: proposed 5 Oct 2026, 23:20; Rae Okafor asked for changes on 5 Oct 2026, 23:40\n" +
				"  Revision 1 change request:\n  > Drop the old endpoint.\n  > Nothing reads it."
		);
		expect(text).toContain("- Revision 2 (this copy): proposed 6 Oct 2026, 00:00; Waiting for approval");
		expect(text).not.toContain("answers");
	});

	it("copies the older revision that is open and says a newer one exists", () => {
		const first = plan({ revision: 1, body: "## Old body" });
		const second = plan({ revision: 2, body: "## New body", proposedAt: "2026-10-05T20:00:00Z" });
		const text = reviewText(input({ plan: first, plans: [first, second] }));

		expect(text).toContain("## Plan — Revision 1 of 2");
		expect(text).toContain("### Plan text\n\n## Old body");
		expect(text).not.toContain("## New body");
		expect(text).toContain(
			"> A newer revision exists: Revision 2, proposed 6 Oct 2026, 00:00. This copy is of Revision 1."
		);
		expect(text).toContain("- Status: Replaced by a newer revision");
	});

	it("quotes the feedback left on the copied revision itself under its status", () => {
		const first = plan({
			revision: 1,
			revisionFeedback: "Too broad.",
			revisionRequestedByName: "Rae Okafor",
			revisionRequestedAt: "2026-10-05T19:40:00Z",
		});
		const text = reviewText(input({ plan: first, plans: [first] }));

		expect(text).toContain("- Status: Rae Okafor asked for changes on 5 Oct 2026, 23:40\n\n> Too broad.");
	});
});
