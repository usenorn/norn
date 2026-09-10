import { flushSync, mount, unmount } from "svelte";
import { describe, expect, it, vi } from "vitest";
import Diagnostics from "./diagnostics.svelte";
import { copyRefusedLine } from "$lib/clipboard";

const entries = [
	{ key: "error", value: "sso_metadata_unreachable" },
	{ key: "provider", value: "Okta · SAML 2.0" },
];

const raised: { message: string; tone?: string }[] = [];

vi.mock("$lib/toast/toasts", () => ({
	showToast: (message: string) => raised.push({ message }),
	showFailure: (message: string) => raised.push({ message, tone: "failure" }),
}));

function shown(writeText: (text: string) => Promise<void>) {
	Object.defineProperty(navigator, "clipboard", {
		configurable: true,
		value: { writeText },
	});

	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(Diagnostics, {
		target,
		props: { label: "Provider response", entries },
	});

	flushSync();

	return { target, held };
}

function copy(target: HTMLElement) {
	const button = [...target.querySelectorAll("button")].find((one) =>
		one.textContent?.includes("Copy")
	);

	if (!button) throw new Error("no copy button");

	button.click();
	flushSync();
}

async function settled() {
	await new Promise((wake) => setTimeout(wake, 0));

	flushSync();
}

describe("copying a diagnostic", () => {
	it("copies the lines it shows and says so once the write lands", async () => {
		raised.length = 0;

		let copied = "";
		const { target, held } = shown(async (text) => {
			copied = text;
		});

		copy(target);
		await settled();

		expect(copied).toBe("error: sso_metadata_unreachable\nprovider: Okta · SAML 2.0");
		expect(raised).toEqual([{ message: "Copied provider response" }]);

		unmount(held);
		target.remove();
	});

	it("says the browser refused rather than claiming it copied", async () => {
		raised.length = 0;

		const { target, held } = shown(async () => {
			throw new Error("denied");
		});

		copy(target);
		await settled();

		expect(raised).toEqual([{ message: copyRefusedLine, tone: "failure" }]);
		expect(target.textContent).toContain("sso_metadata_unreachable");

		unmount(held);
		target.remove();
	});
});
