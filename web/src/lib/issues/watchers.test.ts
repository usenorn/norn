import { flushSync, mount, unmount } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import WatchersHost from "../../../test/watchers-host.svelte";
import type { PersonDetail } from "#lib/workspace/members.js";

const people: PersonDetail[] = Array.from({ length: 7 }, (_, index) => ({
	accountId: `00000000-0000-4000-8000-00000000020${index}`,
	name: `Person ${index}`,
	standing: index === 6 ? "Agent" : "Member",
	email: index === 6 ? undefined : `person${index}@northwind.co`,
}));

const mounted: { target: HTMLElement; held: ReturnType<typeof mount> }[] = [];

function shown(watchers: PersonDetail[]): HTMLElement {
	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(WatchersHost, { target, props: { watchers } });

	flushSync();
	mounted.push({ target, held });

	return target;
}

function press(button: HTMLElement, pointerType: string) {
	const down = new MouseEvent("pointerdown", { bubbles: true });

	Object.defineProperty(down, "pointerType", { value: pointerType });

	button.dispatchEvent(down);
	button.dispatchEvent(new MouseEvent("pointerup", { bubbles: true }));
	button.click();
	flushSync();
}

async function settled() {
	await new Promise((resolve) => setTimeout(resolve, 0));
	flushSync();
}

function onScreen(text: string): boolean {
	return document.body.textContent?.includes(text) ?? false;
}

function avatar(target: HTMLElement, name: string): HTMLElement {
	return target.querySelector(`button[aria-label="${name}"]`) as HTMLElement;
}

afterEach(() => {
	for (const { target, held } of mounted.splice(0)) {
		void unmount(held);
		target.remove();
	}
});

describe("the people watching an issue", () => {
	it("opens a watcher's details on a tap, where there is no hover, and closes them on the next", async () => {
		const button = avatar(shown(people.slice(0, 1)), "Person 0");

		press(button, "touch");
		await settled();
		expect(onScreen("person0@northwind.co")).toBe(true);

		press(button, "touch");
		await settled();
		expect(onScreen("person0@northwind.co")).toBe(false);
	});

	it("leaves a mouse click alone, because the pointer already had the hover", async () => {
		press(avatar(shown(people.slice(0, 1)), "Person 0"), "mouse");
		await settled();

		expect(onScreen("person0@northwind.co")).toBe(false);
	});

	it("stops at five faces and counts the rest", () => {
		const target = shown(people);

		expect(target.querySelectorAll("[data-slot='avatar']")).toHaveLength(5);
		expect(target.textContent).toContain("+2");
	});

	it("lists exactly the people the count hides, with the same details", async () => {
		const target = shown(people);

		target.querySelector<HTMLElement>("[data-slot='popover-trigger']")?.click();
		flushSync();
		await settled();

		expect(onScreen("person5@northwind.co")).toBe(true);
		expect(onScreen("Agent")).toBe(true);
		expect(onScreen("person4@northwind.co")).toBe(false);
	});

	it("offers no count while everybody fits", () => {
		expect(shown(people.slice(0, 5)).querySelector("[data-slot='popover-trigger']")).toBeNull();
	});

	it("says nobody is watching when nobody is", () => {
		expect(shown([]).textContent).toContain("Nobody yet");
	});
});
