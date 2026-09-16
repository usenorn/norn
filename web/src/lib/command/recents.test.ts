import { afterEach, describe, expect, it, vi } from "vitest";
import { readRecents, recentLimit, referenceOf, rememberRecent, resolveRecents } from "./recents";

function memoryStorage(): Storage {
	const items = new Map<string, string>();

	return {
		get length() {
			return items.size;
		},
		clear: () => items.clear(),
		getItem: (key) => items.get(key) ?? null,
		key: (index) => [...items.keys()][index] ?? null,
		removeItem: (key) => void items.delete(key),
		setItem: (key, value) => void items.set(key, value),
	};
}

afterEach(() => vi.unstubAllGlobals());

describe("recents", () => {
	it("stores only the id and href of what was chosen, newest first and without repeats", () => {
		const storage = memoryStorage();
		vi.stubGlobal("localStorage", storage);

		rememberRecent("w1", { id: "a", href: "/w/a" });
		rememberRecent("w1", { id: "b", href: "/w/b" });
		rememberRecent("w1", { id: "a", href: "/w/a", label: "Leaked" } as never);

		expect(readRecents("w1")).toEqual([
			{ id: "a", href: "/w/a" },
			{ id: "b", href: "/w/b" },
		]);
		expect(storage.getItem("norn.palette.recent.w1")).not.toContain("Leaked");
	});

	it("keeps a bounded list", () => {
		vi.stubGlobal("localStorage", memoryStorage());

		for (let n = 0; n < recentLimit + 3; n++) rememberRecent("w1", { id: `${n}`, href: `/w/${n}` });

		expect(readRecents("w1")).toHaveLength(recentLimit);
	});

	it("works as an empty list when storage is blocked or holds garbage", () => {
		vi.stubGlobal("localStorage", {
			getItem: () => {
				throw new Error("blocked");
			},
			setItem: () => {
				throw new Error("blocked");
			},
		});

		expect(readRecents("w1")).toEqual([]);
		expect(rememberRecent("w1", { id: "a", href: "/w/a" })).toEqual([{ id: "a", href: "/w/a" }]);

		vi.stubGlobal("localStorage", {
			getItem: () => '[{"id":1},{"id":"x","href":"https://evil"},{"id":"y","href":"//evil"},{"id":"z","href":"/\\\\evil"}]',
		});

		expect(readRecents("w1")).toEqual([]);
	});

	it("drops a recent item that no longer resolves to something the member can reach", () => {
		const inbox = { id: "nav:go-inbox", label: "Inbox", href: "/w/inbox" };

		expect(
			resolveRecents(
				[
					{ id: "nav:go-inbox", href: "/w/inbox" },
					{ id: "project:gone", href: "/w/projects/gone" },
				],
				[inbox],
				new Map()
			)
		).toEqual([inbox]);
	});

	it("opens the address the workspace has now, not the one this device remembered", () => {
		const team = { id: "team-issues:t1", label: "Issues", href: "/w/teams/NEW/issues" };

		expect(
			resolveRecents([{ id: "team-issues:t1", href: "/w/teams/OLD/issues" }], [team], new Map())
		).toEqual([team]);
	});

	it("reads the issue reference out of an issue href", () => {
		expect(referenceOf("/w/issues/MOB-241?comment=c1#comment-c1")).toBe("MOB-241");
		expect(referenceOf("/w/projects/mobile")).toBeNull();
	});
});
