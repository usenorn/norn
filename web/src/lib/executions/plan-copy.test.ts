import { flushSync, mount, unmount } from "svelte";
import { describe, expect, it, vi } from "vitest";
import { copyRefusedLine } from "$lib/clipboard";
import PlanCopy from "./plan-copy.svelte";
import PlanPanel from "./plan-panel.svelte";
import type { Execution } from "./executions";
import { copiedPlanLine, questionsMissingLine } from "./plan-review";
import type { ExecutionPlan } from "./plans";

const raised: { message: string; tone?: string }[] = [];

vi.mock("$lib/toast/toasts", () => ({
	showToast: (message: string) => raised.push({ message }),
	showFailure: (message: string) => raised.push({ message, tone: "failure" }),
}));

function clipboard(writeText: (text: string) => Promise<void>) {
	Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
}

function mounted<T extends Record<string, unknown>>(component: typeof PlanCopy | typeof PlanPanel, props: T) {
	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(component as typeof PlanCopy, { target, props: props as never });

	flushSync();

	return {
		target,
		done() {
			unmount(held);
			target.remove();
		},
	};
}

function button(target: HTMLElement, name: string): HTMLButtonElement {
	const found = [...target.querySelectorAll("button")].find((one) => one.textContent?.trim() === name);

	if (!found) throw new Error(`no button named ${name}`);

	return found;
}

async function settled() {
	await new Promise((wake) => setTimeout(wake, 0));

	flushSync();
}

function plan(fields: Partial<ExecutionPlan> & { revision: number }): ExecutionPlan {
	return {
		id: `plan-${fields.revision}`,
		executionId: "exec-45",
		body: `## Body of revision ${fields.revision}`,
		proposedAt: "2026-10-05T19:20:00Z",
		...fields,
	};
}

const execution = {
	id: "exec-45",
	reference: "MOO-45",
	issueReference: "MOO-45",
	issueTitle: "Filters",
	state: "awaiting_plan_approval",
	stage: "planning",
} as Execution;

const links = { issue: "https://norn.test/issues/MOO-45", run: "https://norn.test/executions/exec-45" };

describe("copying a plan for review", () => {
	it("puts the text on the clipboard and reads Copied once the write lands", async () => {
		raised.length = 0;

		let copied = "";

		clipboard(async (text) => {
			copied = text;
		});

		const { target, done } = mounted(PlanCopy, { text: "# MOO-45\n\nthe plan" });

		button(target, "Copy for review").click();
		await settled();

		expect(copied).toBe("# MOO-45\n\nthe plan");
		expect(raised).toEqual([{ message: copiedPlanLine }]);
		expect(button(target, "Copied")).toBeTruthy();
		expect(target.querySelector("textarea")).toBeNull();

		done();
	});

	it("shows the whole text in a field when the browser refuses the clipboard", async () => {
		raised.length = 0;

		clipboard(async () => {
			throw new Error("denied");
		});

		const { target, done } = mounted(PlanCopy, { text: "# MOO-45\n\nthe plan" });

		button(target, "Copy for review").click();
		await settled();

		expect(raised).toEqual([{ message: copyRefusedLine, tone: "failure" }]);
		expect(target.querySelector("textarea")?.value).toBe("# MOO-45\n\nthe plan");
		expect(button(target, "Copy for review")).toBeTruthy();

		done();
	});

	it("is disabled with the reason when the questions did not load", () => {
		const { target, done } = mounted(PlanCopy, { text: "x", blocked: questionsMissingLine });
		const copy = button(target, "Copy for review");

		expect(copy.disabled).toBe(true);
		expect(target.textContent).toContain(questionsMissingLine);
		expect(document.getElementById(copy.getAttribute("aria-describedby") ?? "")?.textContent).toContain(
			"did not load"
		);

		done();
	});
});

describe("copying from the plan panel", () => {
	it("copies the revision that is open, and switching revisions clears Copied and copies the other one", async () => {
		raised.length = 0;

		const copied: string[] = [];

		clipboard(async (text) => {
			copied.push(text);
		});

		const first = plan({ revision: 1 });
		const second = plan({ revision: 2, proposedAt: "2026-10-05T20:00:00Z" });
		const { target, done } = mounted(PlanPanel, {
			execution,
			plans: [first, second],
			plansReach: "loaded",
			questions: [],
			questionsReach: "loaded",
			right: { canDecide: false },
			timezone: "UTC",
			links,
			working: false,
			onapprove: () => {},
			onrevise: async () => true,
			onretry: () => {},
		});

		const shown = () => target.querySelector('[role="tabpanel"]:not([hidden])') as HTMLElement;

		button(shown(), "Copy for review").click();
		await settled();

		expect(copied).toHaveLength(1);
		expect(copied[0]).toContain("## Plan — Revision 2 of 2");
		expect(copied[0]).toContain("## Body of revision 2");
		expect(button(shown(), "Copied")).toBeTruthy();

		const trigger = [...target.querySelectorAll('[role="tab"]')].find(
			(one) => one.textContent?.trim() === "Revision 1"
		) as HTMLElement;

		trigger.click();
		flushSync();
		await settled();

		expect(() => button(shown(), "Copied")).toThrow();

		button(shown(), "Copy for review").click();
		await settled();

		expect(copied).toHaveLength(2);
		expect(copied[1]).toContain("## Plan — Revision 1 of 2");
		expect(copied[1]).toContain("## Body of revision 1");
		expect(copied[1]).not.toContain("## Body of revision 2");

		done();
	});

	it("offers to try again when the plans could not be loaded, instead of saying there is no plan", () => {
		let retried = 0;
		const { target, done } = mounted(PlanPanel, {
			execution,
			plans: [],
			plansReach: "unavailable",
			questions: [],
			questionsReach: "loaded",
			right: { canDecide: false },
			timezone: "UTC",
			links,
			working: false,
			onapprove: () => {},
			onrevise: async () => true,
			onretry: () => (retried += 1),
		});

		expect(target.textContent).toContain("The plan could not be loaded.");
		expect(target.textContent).not.toContain("writing a plan");

		button(target, "Try again").click();

		expect(retried).toBe(1);

		done();
	});
});
