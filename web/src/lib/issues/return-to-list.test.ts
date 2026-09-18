import { describe, expect, it } from "vitest";
import { listReturn, withReturn } from "./return-to-list";

const fallback = "/acme/issues";

function backFrom(from: string | null): string {
	const url = new URL("https://norn.test/acme/issues/NORN-209");

	if (from !== null) url.searchParams.set("from", from);

	return listReturn(url, "acme", fallback);
}

describe("withReturn", () => {
	it("keeps the query the issue link already carries", () => {
		expect(withReturn("/acme/issues/NORN-209?tab=activity", "/acme/issues?assignee=eric")).toBe(
			"/acme/issues/NORN-209?tab=activity&from=%2Facme%2Fissues%3Fassignee%3Deric"
		);
	});
});

describe("listReturn", () => {
	it("returns the list the issue was opened from", () => {
		expect(backFrom("/acme/teams/NORN/issues?assignee=eric&group=status")).toBe(
			"/acme/teams/NORN/issues?assignee=eric&group=status"
		);
		expect(backFrom("/acme/issues?assignee=eric")).toBe("/acme/issues?assignee=eric");
		expect(backFrom("/acme/my-tasks?due=today")).toBe("/acme/my-tasks?due=today");
	});

	it("refuses anything but a list of this workspace", () => {
		for (const from of [
			"https://evil.test/acme/issues",
			"//evil.test/acme/issues",
			"/other/issues?assignee=eric",
			"/acme/settings/members",
			"/acme/issues/NORN-1",
			"/acme/teams/NORN/issues/extra",
			"issues",
		]) {
			expect({ from, back: backFrom(from) }).toEqual({ from, back: fallback });
		}
	});

	it("falls back when the issue was not opened from a list", () => {
		expect(backFrom(null)).toBe(fallback);
		expect(backFrom("")).toBe(fallback);
	});
});
