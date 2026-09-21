import { isTypingTarget } from "./shortcuts";

export type EarlyKey = { key: string; at: number };

const listKeys = new Set(["j", "k", "x", " ", "arrowup", "arrowdown"]);

const limit = 8;
const freshness = 1000;
const expiry = 3000;

let queued: EarlyKey[] = [];
let release: (() => void) | undefined;

function typing(target: EventTarget | null): boolean {
	if (isTypingTarget(target)) return true;

	return target instanceof HTMLElement && Boolean(target.closest('[role="textbox"]'));
}

function shown(element: Element): boolean {
	if (element.hasAttribute("hidden")) return false;
	if (element.getAttribute("aria-hidden") === "true") return false;

	const visible = (element as { checkVisibility?: () => boolean }).checkVisibility;

	return visible ? visible.call(element) : true;
}

function dialogOpen(): boolean {
	if (document.querySelector("dialog[open]")) return true;

	return [...document.querySelectorAll('[role="dialog"]')].some(shown);
}

function wanted(event: KeyboardEvent): boolean {
	if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return false;
	if (event.repeat || event.isComposing) return false;
	if (!listKeys.has(event.key.toLowerCase())) return false;
	if (typing(event.target)) return false;
	if (!document.querySelector("[data-issue]")) return false;

	return !dialogOpen();
}

export function startEarlyKeys() {
	if (release) return;

	const take = (event: KeyboardEvent) => {
		if (queued.length >= limit || !wanted(event)) return;

		event.preventDefault();
		event.stopImmediatePropagation();

		queued.push({ key: event.key, at: performance.now() });
	};

	const expires = setTimeout(() => stopEarlyKeys(), expiry);

	window.addEventListener("keydown", take, true);

	release = () => {
		clearTimeout(expires);
		window.removeEventListener("keydown", take, true);
	};
}

export function stopEarlyKeys() {
	release?.();
	release = undefined;
	queued = [];
}

export function drainEarlyKeys(handle: (event: KeyboardEvent) => void) {
	const taken = queued;

	release?.();
	release = undefined;
	queued = [];

	const now = performance.now();

	for (const early of taken) {
		if (now - early.at > freshness) continue;

		handle(new KeyboardEvent("keydown", { key: early.key }));
	}
}
