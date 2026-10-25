import { describe, expect, it } from "vitest";
import type { Project } from "#lib/projects/projects.js";
import {
	choicesFor,
	evidenceLink,
	memoriesPath,
	memoryAuthorLine,
	memoryContentFields,
	memoryContentOf,
	memoryFacetChips,
	memoryHold,
	memoryListParams,
	memoryQueryOf,
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
		viewer: { canEdit: true, canShare: true, canReview: false },
		createdAt: "2026-10-25T09:00:00Z",
		updatedAt: "2026-10-25T09:00:00Z",
	};
}

describe("memory filters", () => {
	const read = (path: string) => memoryQueryOf(new URL(`https://norn.test${path}`).searchParams);

	it("defaults to the accepted memories agents recall", () => {
		expect(memoryQueryOf(new URLSearchParams())).toEqual({ tab: "accepted" });
		expect(memoryQueryOf(new URLSearchParams("tab=bogus&scope=everyone&kind=rumour"))).toEqual({ tab: "accepted" });
		expect(memoriesPath("acme")).toBe("/acme/memories");
	});

	it("reads back every filter the link wrote", () => {
		const query = { tab: "all", scope: "project", kind: "lesson", status: "withdrawn", project: "p1", component: "ledger" } as const;

		expect(read(memoriesPath("acme", query))).toEqual(query);
		expect(memoriesPath("acme", { tab: "review" }, "c1")).toBe("/acme/memories?tab=review&cursor=c1");
	});

	it("keeps a status only on the tab that offers it", () => {
		expect(read(memoriesPath("acme", { tab: "accepted", status: "withdrawn" }))).toEqual({ tab: "accepted" });
		expect(memoryQueryOf(new URLSearchParams("tab=review&status=rejected"))).toEqual({ tab: "review" });
	});

	it("asks the API for the queue the viewer may decide on", () => {
		expect(memoryListParams({ tab: "review", scope: "workspace" })).toEqual({ reviewable: true, visibility: ["workspace"] });
		expect(memoryListParams({ tab: "accepted", kind: "constraint" })).toEqual({ status: ["accepted"], kind: ["constraint"] });
		expect(memoryListParams({ tab: "all" })).toEqual({});
		expect(memoryListParams({ tab: "all", status: "rejected", project: "p1", component: "ledger" })).toEqual({
			status: ["rejected"],
			projectId: "p1",
			component: "ledger",
		});
	});

	it("names each chosen filter and links to the list without it", () => {
		const projects = [{ id: "p1", name: "Billing" }] as Project[];
		const chips = memoryFacetChips("acme", { tab: "accepted", scope: "personal", project: "p1" }, projects);

		expect(chips.map((chip) => chip.label)).toEqual(["Shared with: Only me", "Applies to: Billing"]);
		expect(chips[0].removed).toBe("/acme/memories?project=p1");
	});
});

describe("where a memory came from", () => {
	it("names the agent and the person it acted for", () => {
		expect(memoryAuthorLine({ authorName: "Rae", agentName: "Scout", source: "agent" })).toBe("Scout for Rae");
		expect(memoryAuthorLine({ authorName: "Rae", source: "agent" })).toBe("An agent for Rae");
		expect(memoryAuthorLine({ authorName: "Rae", source: "person" })).toBe("Rae");
		expect(memoryAuthorLine({ source: "import" })).toBe("Somebody");
	});

	it("links cited issues by reference and doc versions by revision", () => {
		expect(evidenceLink("acme", { kind: "issue", issueId: "i1", issueReference: "NORN-7", issueTitle: "Fix rounding" })).toEqual({
			label: "Fix rounding",
			detail: "An issue",
			href: "/acme/issues/NORN-7",
		});
		expect(evidenceLink("acme", { kind: "doc_revision", docId: "d1", docRevisionId: "r1", docTitle: "Ledger" }).href).toBe(
			"/acme/docs/d1/history?revision=r1"
		);
		expect(evidenceLink("acme", { kind: "excerpt", excerpt: "it broke" })).toEqual({ label: "A pasted excerpt", quote: "it broke" });
	});
});

describe("a correction", () => {
	it("round-trips the content and keeps the evidence without its display fields", () => {
		const cited = memory("m1", "accepted", { projectId: "p1", component: "ledger" });
		cited.evidence = [{ kind: "issue", issueId: "i1", issueReference: "NORN-7", issueTitle: "Fix rounding" }];

		const content = memoryContentOf(memoryContentFields(cited), cited.evidence);

		expect(content.appliesTo).toEqual({ projectId: "p1", component: "ledger" });
		expect(content.evidence).toEqual([
			{ kind: "issue", issueId: "i1", commentId: undefined, docId: undefined, docRevisionId: undefined, excerpt: undefined },
		]);
		expect(content.effectiveFrom).toBe("2026-10-25");
		expect(memoryContentOf({ ...memoryContentFields(cited), appliesTo: "workspace", component: "" }).appliesTo).toEqual({});
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
		expect(readMemoryFailure({ reason: "memory_review_forbidden", status: 403 })).toEqual({ kind: "not_reviewer" });
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
