import { describe, expect, it } from "vitest";
import { emoji, matchingEmoji } from "./emoji";
import { opensMenu } from "./slash";

describe("finding an emoji by name", () => {
	it("puts the exact name first", () => {
		expect(matchingEmoji("tada")[0].glyph).toBe("🎉");
		expect(matchingEmoji("fire")[0].glyph).toBe("🔥");
	});

	it("answers a part of a name, which is what somebody typing sees", () => {
		expect(matchingEmoji("roc")[0].glyph).toBe("🚀");
		expect(matchingEmoji("thumbsu")[0].glyph).toBe("👍");
	});

	it("answers a word somebody would think of rather than the official name", () => {
		expect(matchingEmoji("party").map((one) => one.glyph)).toContain("🎉");
		expect(matchingEmoji("ship").map((one) => one.glyph)).toContain("🚀");
		expect(matchingEmoji("bug").map((one) => one.glyph)).toContain("🐛");
	});

	it("offers something to start from before anything is typed", () => {
		expect(matchingEmoji("").length).toBeGreaterThan(0);
	});

	it("returns nothing rather than everything for a name nobody has", () => {
		expect(matchingEmoji("qwertyuiop")).toEqual([]);
	});

	it("keeps the list short enough to read", () => {
		expect(matchingEmoji("a").length).toBeLessThanOrEqual(8);
	});

	it("names every emoji once, so the popup has no duplicate keys", () => {
		const names = emoji.map((one) => one.name);

		expect(new Set(names).size).toBe(names.length);
	});
});

describe("when the colon opens the emoji list", () => {
	it("opens at the start of a line and after a space or a bracket", () => {
		expect(opensMenu("")).toBe(true);
		expect(opensMenu(" ")).toBe(true);
		expect(opensMenu("(")).toBe(true);
	});

	it("stays shut inside a word, so a web address and a time are left alone", () => {
		expect(opensMenu("p")).toBe(false);
		expect(opensMenu("2")).toBe(false);
		expect(opensMenu("y")).toBe(false);
	});
});
