import { describe, expect, it } from "vitest";
import type { Project } from "$lib/projects/projects";
import { ancestry, descendants, docTree, readDocFailure, revisionAuthor, type DocSummary } from "./docs";

function summary(id: string, title: string, audience: DocSummary["audience"], parentId?: string): DocSummary {
	return {
		id,
		title,
		audience,
		parentId,
		status: "active",
		version: 1,
		createdAt: "2026-10-01T09:00:00Z",
		updatedAt: "2026-10-01T09:00:00Z",
	};
}

const projects = [{ id: "p1", name: "Checkout rebuild" }] as Project[];

describe("docTree", () => {
	it("groups top-level docs by who reads them and files children under their parent", () => {
		const tree = docTree(
			[
				summary("b", "Zeta", { visibility: "workspace" }),
				summary("a", "Alpha", { visibility: "workspace" }),
				summary("c", "Notes", { visibility: "personal" }),
				summary("d", "Plan", { visibility: "project", projectId: "p1" }),
				summary("e", "Child", { visibility: "personal" }, "a"),
			],
			projects
		);

		if (tree.kind !== "ready") throw new Error(`tree is ${tree.kind}`);

		expect(tree.sections.map((section) => section.title)).toEqual(["Workspace", "Checkout rebuild", "Personal"]);
		expect(tree.sections[0].nodes.map((node) => node.doc.title)).toEqual(["Alpha", "Zeta"]);
		expect(tree.sections[0].nodes[0].children.map((node) => node.doc.id)).toEqual(["e"]);
	});

	it("puts a doc whose parent the reader cannot see at the top of its own audience", () => {
		const tree = docTree([summary("x", "Orphan", { visibility: "workspace" }, "hidden")], projects);

		if (tree.kind !== "ready") throw new Error(`tree is ${tree.kind}`);

		expect(tree.sections[0].nodes[0].doc.id).toBe("x");
	});

	it("is empty when nothing is shared with the reader", () => {
		expect(docTree([], projects).kind).toBe("empty");
	});
});

describe("ancestry and descendants", () => {
	const docs = [
		summary("a", "A", { visibility: "workspace" }),
		summary("b", "B", { visibility: "workspace" }, "a"),
		summary("c", "C", { visibility: "workspace" }, "b"),
	];

	it("walks from the root down to the parent", () => {
		expect(ancestry(docs, "c").map((doc) => doc.id)).toEqual(["a", "b"]);
	});

	it("finds everything beneath a doc so it cannot be moved under itself", () => {
		expect([...descendants(docs, "a")].sort()).toEqual(["b", "c"]);
	});
});

describe("readDocFailure", () => {
	it("carries the current version of a stale write", () => {
		expect(readDocFailure({ status: 409, code: "doc_stale", version: 4 })).toEqual({ kind: "stale", version: 4 });
	});

	it("tells a refused edit from a refused reshare", () => {
		expect(readDocFailure({ status: 403, code: "forbidden", reason: "knowledge_not_editable" }).kind).toBe(
			"not_editable"
		);
		expect(readDocFailure({ status: 403, code: "forbidden", reason: "knowledge_not_shareable" }).kind).toBe(
			"not_shareable"
		);
	});

	it("names the fields a validation failure points at", () => {
		expect(readDocFailure({ status: 422, errors: [{ field: "title" }] })).toEqual({
			kind: "invalid",
			fields: ["title"],
		});
	});

	it("treats anything unreadable as a failure that changed nothing", () => {
		expect(readDocFailure(undefined).kind).toBe("unavailable");
	});
});

describe("revisionAuthor", () => {
	it("names the agent and the person it acted for", () => {
		const revision = {
			id: "r",
			docId: "a",
			version: 2,
			action: "edited",
			title: "A",
			audience: { visibility: "workspace" },
			source: "agent",
			agentId: "g1",
			accountName: "Rae Okafor",
			createdAt: "2026-10-01T09:00:00Z",
		} as const;

		expect(revisionAuthor(revision, [{ id: "g1", name: "Scribe" }] as never)).toBe("Scribe for Rae Okafor");
		expect(revisionAuthor(revision, [])).toBe("An agent for Rae Okafor");
	});
});
