import { flushSync, mount, unmount } from "svelte";
import { describe, expect, it } from "vitest";
import DecisionActions from "./decision-actions.svelte";

const copy = {
	approveLabel: "Approve the plan",
	confirmPrompt: "Build this plan?",
	confirmLabel: "Approve it",
	requestLabel: "Ask for changes",
	feedbackLabel: "What should change",
	feedbackPlaceholder: "Say what should change.",
	sendLabel: "Send it back",
	sendHint: "The coding agent revises the plan.",
	feedbackMaxLength: 4000,
};

function press(target: HTMLElement, name: string) {
	const button = [...target.querySelectorAll("button")].find(
		(one) => one.textContent?.trim() === name
	);

	if (!button) throw new Error(`no button named ${name}`);

	button.click();
	flushSync();
}

function said(target: HTMLElement): HTMLTextAreaElement {
	const field = target.querySelector("textarea");

	if (!field) throw new Error("the feedback field is not open");

	return field;
}

function typed(target: HTMLElement, words: string) {
	const field = said(target);

	field.value = words;
	field.dispatchEvent(new Event("input", { bubbles: true }));
	flushSync();
}

async function settled() {
	await new Promise((wake) => setTimeout(wake, 0));

	flushSync();
}

function mounted(props: { onrequest?: (feedback: string) => Promise<boolean>; blocked?: string }) {
	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(DecisionActions, {
		target,
		props: {
			...copy,
			working: false,
			blocked: props.blocked,
			onapprove: () => {},
			onrequest: props.onrequest ?? (async () => true),
		},
	});

	flushSync();

	return { target, held };
}

function opened(onrequest: (feedback: string) => Promise<boolean>) {
	const { target, held } = mounted({ onrequest });

	press(target, "Ask for changes");

	return { target, held };
}

describe("deciding while something still blocks approval", () => {
	it("keeps approval out of reach and says why, but still lets changes be asked for", () => {
		const { target, held } = mounted({ blocked: "Answer the open question before approving." });

		const approve = [...target.querySelectorAll("button")].find(
			(button) => button.textContent?.trim() === "Approve the plan"
		);

		expect(approve?.disabled).toBe(true);
		expect(target.textContent).toContain("Answer the open question before approving.");

		press(target, "Ask for changes");
		expect(target.querySelector("textarea")).not.toBe(null);

		unmount(held);
		target.remove();
	});
});

describe("asking for changes", () => {
	it("keeps the words when the request is refused", async () => {
		let asked = "";

		const { target, held } = opened(async (feedback) => {
			asked = feedback;

			return false;
		});

		typed(target, "Rename the endpoint.");
		press(target, "Send it back");
		await settled();

		expect(asked).toBe("Rename the endpoint.");
		expect(said(target).value).toBe("Rename the endpoint.");

		unmount(held);
		target.remove();
	});

	it("closes the panel only once the run has taken them", async () => {
		const { target, held } = opened(async () => true);

		typed(target, "Rename the endpoint.");
		press(target, "Send it back");
		await settled();

		expect(target.querySelector("textarea")).toBe(null);

		unmount(held);
		target.remove();
	});
});
