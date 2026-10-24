import { describe, expect, it } from "vitest";
import type { Project } from "$lib/projects/projects";
import type { IssueCandidate } from "$lib/issues/issues";
import {
	ancestry,
	descendants,
	docTree,
	readDocFailure,
	revisionAuthor,
	unlinkedIssues,
	unlinkedProjects,
	type DocLink,
	type DocSummary,
} from "./docs";

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

	it("names a link the doc already holds", () => {
		expect(readDocFailure({ status: 409, code: "doc_link_exists" })).toEqual({ kind: "link_exists" });
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

describe("unlinked choices", () => {
	const links: DocLink[] = [
		{
			id: "l1",
			target: "issue",
			issue: { id: "i1", reference: "OPS-1", title: "Linked", state: { name: "Todo", category: "not_started" } },
			createdAt: "2026-10-01T09:00:00Z",
		},
		{
			id: "l2",
			target: "project",
			project: { id: "p1", slug: "checkout", name: "Checkout rebuild", archived: false },
			createdAt: "2026-10-01T09:00:00Z",
		},
	];

	it("offers only the issues the doc does not already link to", () => {
		const candidates = [{ id: "i1" }, { id: "i2" }] as IssueCandidate[];

		expect(unlinkedIssues(links, candidates).map((candidate) => candidate.id)).toEqual(["i2"]);
	});

	it("offers only the projects the doc does not already link to", () => {
		const offered = unlinkedProjects(links, [...projects, { id: "p2", name: "Billing" }] as Project[]);

		expect(offered.map((project) => project.id)).toEqual(["p2"]);
	});
});
