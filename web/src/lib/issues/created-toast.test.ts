import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Raised } from "$lib/toast/toasts";
import type { Issue } from "./issues";
import { announceCreated } from "./created-toast";

const raised: { message: string; options: Raised }[] = [];
const copied: { text: string; said: string }[] = [];

vi.mock("$lib/toast/toasts", () => ({
	showToast: (message: string, options: Raised) => raised.push({ message, options }),
}));

vi.mock("$lib/clipboard", () => ({
	copyText: async (text: string, said: string) => {
		copied.push({ text, said });

		return true;
	},
}));

function filed(descriptionDoc?: Issue["descriptionDoc"]): Issue {
	return { reference: "NORN-9", description: "", descriptionDoc } as Issue;
}

describe("the toast for a freshly created issue", () => {
	beforeEach(() => {
		raised.length = 0;
		copied.length = 0;
	});

	it("names the issue, opens it, and shows where its description begins", () => {
		announceCreated(
			filed({
				type: "doc",
				content: [
					{ type: "paragraph", content: [{ type: "text", text: "Refunds stall" }] },
					{ type: "paragraph", content: [{ type: "text", text: "after checkout" }] },
				],
			}),
			"/northwind/issues/NORN-9",
			"https://norn.test"
		);

		expect(raised).toHaveLength(1);
		expect(raised[0].message).toBe("Created NORN-9");
		expect(raised[0].options.href).toBe("/northwind/issues/NORN-9");
		expect(raised[0].options.detail).toBe("Refunds stall after checkout");
		expect(raised[0].options.action).toBe("Copy link");
	});

	it("copies a link that works outside the app", async () => {
		announceCreated(filed(), "/northwind/issues/NORN-9", "https://norn.test");

		raised[0].options.onaction?.();
		await Promise.resolve();

		expect(copied).toEqual([
			{ text: "https://norn.test/northwind/issues/NORN-9", said: "Copied a link to NORN-9" },
		]);
	});

	it("says nothing more when the issue has no description", () => {
		announceCreated(
			filed({ type: "doc", content: [] }),
			"/northwind/issues/NORN-9",
			"https://norn.test"
		);

		expect(raised[0].options.detail).toBeUndefined();
	});
});
