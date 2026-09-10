import { flushSync, mount, unmount } from "svelte";
import { describe, expect, it } from "vitest";
import ReviewActions from "./review-actions.svelte";
import type { Execution } from "./executions";

const waiting = { state: "awaiting_review" } as Execution;

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

function opened(onrequestchanges: (feedback: string) => Promise<boolean>) {
	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(ReviewActions, {
		target,
		props: { execution: waiting, working: false, onapprove: () => {}, onrequestchanges },
	});

	flushSync();
	press(target, "Request changes");

	return { target, held };
}

describe("asking a run for changes", () => {
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
