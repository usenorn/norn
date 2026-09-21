import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { drainEarlyKeys, startEarlyKeys, stopEarlyKeys } from "./early-keys";

function press(init: KeyboardEventInit & { on?: EventTarget } = {}) {
	const { on, ...rest } = init;
	const event = new KeyboardEvent("keydown", { key: "j", bubbles: true, cancelable: true, ...rest });

	(on ?? document.body).dispatchEvent(event);

	return event;
}

function drained(): string[] {
	const seen: string[] = [];

	drainEarlyKeys((event) => seen.push(event.key));

	return seen;
}

function listed() {
	document.body.innerHTML = '<div data-issue="one"></div><div data-issue="two"></div>';
}

const listeners: Array<() => void> = [];

function hydrated(): string[] {
	const seen: string[] = [];
	const take = (event: KeyboardEvent) => seen.push(event.key);

	window.addEventListener("keydown", take);
	listeners.push(() => window.removeEventListener("keydown", take));

	return seen;
}

beforeEach(() => {
	vi.useFakeTimers();
	listed();
	startEarlyKeys();
});

afterEach(() => {
	stopEarlyKeys();
	listeners.splice(0).forEach((off) => off());
	vi.useRealTimers();
	document.body.innerHTML = "";
});

describe("what the early buffer takes", () => {
	it("keeps the list keys pressed before the app can answer, in order, once", () => {
		press({ key: "j" });
		press({ key: "x" });

		expect(drained()).toEqual(["j", "x"]);
		expect(drained()).toEqual([]);
	});

	it("stops a taken key from reaching the handler that hydration installs", () => {
		const late = hydrated();
		const event = press({ key: " " });

		expect(late).toEqual([]);
		expect(event.defaultPrevented).toBe(true);
		expect(drained()).toEqual([" "]);
	});

	it("leaves everything it does not own alone", () => {
		const editor = document.createElement("input");
		const textbox = document.createElement("div");
		const dialog = document.createElement("div");

		textbox.setAttribute("role", "textbox");
		dialog.setAttribute("role", "dialog");
		document.body.append(editor, textbox);

		press({ key: "j", on: editor });
		press({ key: "j", on: textbox });
		press({ key: "j", ctrlKey: true });
		press({ key: "j", metaKey: true });
		press({ key: "j", altKey: true });
		press({ key: "j", shiftKey: true });
		press({ key: "j", repeat: true });
		press({ key: "g" });

		document.body.append(dialog);
		press({ key: "j" });
		dialog.remove();

		document.body.innerHTML = "";
		press({ key: "j" });

		expect(drained()).toEqual([]);
	});

	it("ignores a dialog that is in the document but not shown", () => {
		const dialog = document.createElement("div");

		dialog.setAttribute("role", "dialog");
		dialog.setAttribute("aria-hidden", "true");
		document.body.append(dialog);

		press({ key: "x" });

		expect(drained()).toEqual(["x"]);
	});
});

describe("what the early buffer refuses to hold", () => {
	it("takes no more than a handful of keys", () => {
		for (let pressed = 0; pressed < 12; pressed += 1) press({ key: "j" });

		expect(drained()).toHaveLength(8);
	});

	it("drops keys that went stale while the app was loading", () => {
		press({ key: "j" });

		vi.advanceTimersByTime(1500);

		expect(drained()).toEqual([]);
	});

	it("gives the keyboard back when nothing ever drains the queue", () => {
		press({ key: "j" });

		vi.advanceTimersByTime(3000);

		const late = hydrated();
		const event = press({ key: "j" });

		expect(late).toEqual(["j"]);
		expect(event.defaultPrevented).toBe(false);
		expect(drained()).toEqual([]);
	});
});
