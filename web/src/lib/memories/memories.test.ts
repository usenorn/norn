import { describe, expect, it } from "vitest";
import type { Project } from "#lib/projects/projects.js";
import {
	choicesFor,
	memoriesPath,
	memoryFilterOf,
	memoryFilterStatuses,
	memoryHold,
	offers,
	readMemoryFailure,
	scopeLine,
	type Memory,
	type MemoryChoice,
	type RelatedMemory,
} from "./memories";

function memory(id: string, status: Memory["status"], appliesTo: Memory["appliesTo"] = {}): Memory {
	return {
		id,
		kind: "decision",
		statement: "Production runs on PostgreSQL",
		rationale: "",
		conditions: "",
		audience: { visibility: "workspace" },
		appliesTo,
		status,
		basis: "observation",
		evidence: [],
		source: "person",
		effectiveFrom: "2026-10-25",
		version: 1,
		contributorIds: [],
		viewer: { canEdit: true, canShare: true },
		createdAt: "2026-10-25T09:00:00Z",
		updatedAt: "2026-10-25T09:00:00Z",
	};
}

describe("memory filters", () => {
	it("defaults to what needs review and reads back what the link wrote", () => {
		expect(memoryFilterOf(null)).toBe("review");
		expect(memoryFilterOf("bogus")).toBe("review");
		expect(memoryFilterStatuses("review")).toEqual(["proposed", "disputed"]);
		expect(memoryFilterStatuses("all")).toEqual([]);
		expect(memoriesPath("acme")).toBe("/acme/memories");
		expect(memoriesPath("acme", "accepted", "c1")).toBe("/acme/memories?filter=accepted&cursor=c1");
		expect(memoryFilterOf(new URL(`https://norn.test${memoriesPath("acme", "all")}`).searchParams.get("filter"))).toBe("all");
	});
});

describe("a held memory", () => {
	const related: RelatedMemory[] = [{ memory: memory("m1", "accepted"), relation: "conflict" }];
	const choices: MemoryChoice[] = [
		{ resolution: "independent" },
		{ resolution: "update", targetId: "m1" },
		{ resolution: "supersede", targetId: "m1" },
		{ resolution: "no_change" },
	];

	it("is disputed or proposed only while something relates to it", () => {
		expect(memoryHold(memory("m2", "disputed"), related)).toBe("disputed");
		expect(memoryHold(memory("m2", "proposed"), related)).toBe("proposed");
		expect(memoryHold(memory("m2", "proposed"), [])).toBe("settled");
		expect(memoryHold(memory("m2", "accepted"), related)).toBe("settled");
	});

	it("offers targeted choices per related memory and the rest for itself", () => {
		expect(choicesFor(choices, "m1")).toEqual(["update", "supersede"]);
		expect(choicesFor(choices, "m9")).toEqual([]);
		expect(offers(choices, "independent")).toBe(true);
		expect(offers(choices, "supersede")).toBe(false);
	});
});

describe("memory failures", () => {
	it("maps the conflict codes and statuses to copy kinds", () => {
		expect(readMemoryFailure({ code: "memory_stale", version: 4 })).toEqual({ kind: "stale", version: 4 });
		expect(readMemoryFailure({ code: "memory_not_pending" })).toEqual({ kind: "settled" });
		expect(readMemoryFailure({ reason: "knowledge_not_editable", status: 403 })).toEqual({ kind: "not_editable" });
		expect(readMemoryFailure({ status: 404 })).toEqual({ kind: "not_found" });
		expect(readMemoryFailure(undefined)).toEqual({ kind: "unavailable" });
	});
});

describe("scope line", () => {
	const projects = [{ id: "p1", name: "Billing" }] as Project[];

	it("names the project and component a memory applies to", () => {
		expect(scopeLine(memory("m1", "accepted"), projects)).toBe("The whole workspace");
		expect(scopeLine(memory("m1", "accepted", { projectId: "p1", component: "ledger" }), projects)).toBe(
			"Billing · ledger"
		);
	});
});
