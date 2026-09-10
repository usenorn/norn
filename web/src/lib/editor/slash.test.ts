import { describe, expect, it } from "vitest";
import { matchingCommands, opensMenu, slashCommands } from "./slash";

describe("matchingCommands", () => {
	it("offers everything when nothing has been typed", () => {
		expect(matchingCommands("")).toHaveLength(slashCommands.length);
	});

	it("puts an exact key first", () => {
		expect(matchingCommands("todo")[0].key).toBe("todo");
	});

	it("finds a block by a word somebody would actually type", () => {
		expect(matchingCommands("check").map((command) => command.key)).toContain("todo");
		expect(matchingCommands("collapse").map((command) => command.key)).toContain("toggle");
		expect(matchingCommands("ul").map((command) => command.key)).toContain("bullet");
	});

	it("offers nothing rather than everything when nothing matches", () => {
		expect(matchingCommands("qqqq")).toHaveLength(0);
	});

	it("ranks a heading above a block that only mentions the word", () => {
		expect(matchingCommands("h1")[0].key).toBe("h1");
	});

	it("keeps each group in one piece so arrowing down passes through it once", () => {
		const groups = matchingCommands("list").map((command) => command.group);
		const seen = new Set<string>();

		for (let at = 0; at < groups.length; at += 1) {
			if (at > 0 && groups[at] === groups[at - 1]) continue;

			expect(seen.has(groups[at])).toBe(false);
			seen.add(groups[at]);
		}
	});

	it("names the sub-issue command as one that asks before creating work", () => {
		const subissue = slashCommands.find((command) => command.key === "subissue");

		expect(subissue?.confirm).toBeTruthy();
	});
});

describe("opensMenu", () => {
	it("opens at the start of a block", () => {
		expect(opensMenu("")).toBe(true);
	});

	it("opens after a space", () => {
		expect(opensMenu("and then ")).toBe(true);
	});

	it("stays shut inside a path", () => {
		expect(opensMenu("src")).toBe(false);
		expect(opensMenu("web/src")).toBe(false);
	});

	it("stays shut inside a URL", () => {
		expect(opensMenu("https:")).toBe(false);
		expect(opensMenu("https://norn.so")).toBe(false);
	});

	it("opens after an opening bracket or quote", () => {
		expect(opensMenu("(")).toBe(true);
		expect(opensMenu('"')).toBe(true);
	});
});
