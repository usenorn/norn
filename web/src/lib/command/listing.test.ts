import { describe, expect, it } from "vitest";
import type { ShortcutId } from "$lib/shortcuts/shortcuts";
import { paletteCommands } from "./commands";
import { paletteListing, type ListingSources } from "./listing";
import { footNote, modeOf, type CommandScope, type Destination } from "./model";

const scoped: CommandScope = {
	kind: "issues",
	issues: [{ id: "i1", reference: "MOB-241", title: "Offline queue", teamId: "t1" }],
};

const bound = () => true;

const inbox: Destination = { id: "nav:go-inbox", label: "Inbox", href: "/w/inbox", keys: "G N" };
const billing: Destination = { id: "team-issues:t2", label: "Issues", href: "/w/teams/BIL/issues", context: "Billing" };

function sources(overrides: Partial<ListingSources>): ListingSources {
	return {
		mode: { kind: "search", query: "" },
		search: { kind: "idle" },
		recents: [],
		destinations: [inbox, billing],
		commands: [],
		options: [],
		everything: null,
		...overrides,
	};
}

describe("modeOf", () => {
	it("enters command mode on a leading > and strips it from the query", () => {
		expect(modeOf(">assign", null)).toEqual({ kind: "commands", query: "assign" });
	});

	it("filters the open step rather than searching", () => {
		const step = { kind: "assign" as const, label: "Assign MOB-241 to" };

		expect(modeOf(" jun ", step)).toEqual({ kind: "step", step, query: "jun" });
	});
});

describe("paletteCommands", () => {
	it("keeps the global commands when nothing is selected or open, and only those", () => {
		expect(
			paletteCommands({ kind: "none" }, true, new Set(), bound).map((command) => command.id)
		).toEqual([
			"issue-new",
			"open-triage",
			"toggle-density",
		]);
	});

	it("adds every issue action once an issue is in scope", () => {
		expect(
			paletteCommands(scoped, true, new Set(["t1"]), bound).map((command) => command.id)
		).toEqual([
			"issue-new",
			"assign",
			"status",
			"cycle",
			"label",
			"copy-link",
			"open-triage",
			"toggle-density",
		]);
	});

	it("leaves out moving to a cycle when the team runs no cycles", () => {
		expect(
			paletteCommands(scoped, true, new Set(["t2"]), bound).map((command) => command.id)
		).not.toContain("cycle");
	});

	it("only shows the keycap for a shortcut a surface has actually bound", () => {
		const cycle = (binding: (id: ShortcutId) => boolean) =>
			paletteCommands(scoped, true, new Set(["t1"]), binding).find(
				(command) => command.id === "cycle"
			);

		expect(cycle(() => false)?.keys).toBeUndefined();
		expect(cycle((id) => id === "bulk-cycle")?.keys).toBe("⇧ C");
	});
});

describe("paletteListing", () => {
	it("keeps contextual destinations filterable instead of hiding them on the first keystroke", () => {
		const listing = paletteListing(sources({ mode: { kind: "search", query: "billing" }, search: { kind: "searching" } }));

		expect(listing.kind).toBe("searching");
		expect(listing.kind !== "no_matches" && listing.groups[0].entries).toEqual([
			{ kind: "destination", destination: billing },
		]);
	});

	it("reports no matches only when the server and the local places both came up empty", () => {
		const listing = paletteListing(
			sources({ mode: { kind: "search", query: "gantt" }, search: { kind: "ready", groups: [], fuzzy: false } })
		);

		expect(listing).toEqual({ kind: "no_matches", query: "gantt" });
	});

	it("still lists recent items and local places while search is down", () => {
		const listing = paletteListing(sources({ recents: [inbox], search: { kind: "unavailable" } }));

		expect(listing.kind).toBe("unavailable");
		expect(footNote(listing, { kind: "none" })).toBe("degraded · local only");
	});

	it("shows Recent beside the suggested commands rather than instead of them", () => {
		const listing = paletteListing(
			sources({
				recents: [billing],
				commands: [{ id: "issue-new", label: "New issue" }],
			})
		);

		expect(listing.kind !== "no_matches" && listing.groups.map((group) => group.id)).toEqual([
			"recent",
			"suggested",
			"go-to",
		]);
	});

	it("names the issue a command acts on in the footer, and counts commands when there is none", () => {
		const listing = paletteListing(
			sources({
				mode: { kind: "commands", query: "" },
				commands: paletteCommands({ kind: "none" }, true, new Set(), bound),
			})
		);

		expect(footNote(listing, scoped)).toBe("commands act on MOB-241");
		expect(footNote(listing, { kind: "none" })).toBe("3 results");
	});
});
