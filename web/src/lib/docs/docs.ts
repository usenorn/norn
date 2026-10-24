import type { components } from "$lib/api/dashboard.gen";
import type { Agent } from "$lib/agents/agents";
import type { IssueCandidate } from "$lib/issues/issues";
import type { Project } from "$lib/projects/projects";
import { workspacePath } from "$lib/workspace/navigation";

export type Doc = components["schemas"]["Doc"];
export type DocSummary = components["schemas"]["DocSummary"];
export type DocAudience = components["schemas"]["DocAudience"];
export type DocVisibility = components["schemas"]["DocVisibility"];
export type DocRevision = components["schemas"]["DocRevision"];
export type DocRevisionSummary = components["schemas"]["DocRevisionSummary"];
export type DocRevisionAction = components["schemas"]["DocRevisionAction"];
export type DocRevisionSource = components["schemas"]["DocRevisionSource"];
export type DocLink = components["schemas"]["DocLink"];

export const docTreePageSize = 200;
export const docTreePagesMax = 10;
export const docRevisionPageSize = 25;
export const docReadersShown = 8;
export const docTitleMax = 200;

export const docVisibilities: DocVisibility[] = ["workspace", "project", "personal"];

export const docVisibilityLabels: Record<DocVisibility, string> = {
	workspace: "Workspace",
	project: "Project",
	personal: "Only me",
};

export const docAudienceLabels: Record<DocVisibility, string> = {
	workspace: "Everybody in the workspace will be able to read it.",
	project: "The project's members will be able to read it.",
	personal: "Only its author will be able to read it.",
};

export function docsPath(workspace: string): string {
	return workspacePath(workspace, "/docs");
}

export function docPath(workspace: string, docId: string): string {
	return workspacePath(workspace, `/docs/${docId}`);
}

export function docEditPath(workspace: string, docId: string): string {
	return `${docPath(workspace, docId)}/edit`;
}

export function docHistoryPath(workspace: string, docId: string, revisionId?: string): string {
	const path = `${docPath(workspace, docId)}/history`;

	return revisionId ? `${path}?revision=${revisionId}` : path;
}

export function newDocPath(workspace: string, parentId?: string): string {
	const path = workspacePath(workspace, "/docs/new");

	return parentId ? `${path}?parent=${parentId}` : path;
}

export type DocNode = { doc: DocSummary; children: DocNode[] };

export type DocSection = {
	key: string;
	title: string;
	visibility: DocVisibility;
	projectId?: string;
	nodes: DocNode[];
};

export type DocTree =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "empty" }
	| { kind: "ready"; sections: DocSection[]; docs: DocSummary[]; partial: boolean };

const titleOrder = new Intl.Collator("en", { sensitivity: "base", numeric: true });

function byTitle(left: DocNode, right: DocNode): number {
	return titleOrder.compare(left.doc.title, right.doc.title) || left.doc.id.localeCompare(right.doc.id);
}

function sorted(nodes: DocNode[]): DocNode[] {
	return nodes.sort(byTitle).map((node) => ({ ...node, children: sorted(node.children) }));
}

function sectionKey(audience: DocAudience): string {
	return audience.visibility === "project" ? `project:${audience.projectId ?? ""}` : audience.visibility;
}

export function docTree(docs: DocSummary[], projects: Project[], partial = false): DocTree {
	if (docs.length === 0) return { kind: "empty" };

	const nodes = new Map(docs.map((doc) => [doc.id, { doc, children: [] as DocNode[] }]));
	const roots: DocNode[] = [];

	for (const node of nodes.values()) {
		const parent = node.doc.parentId ? nodes.get(node.doc.parentId) : undefined;

		if (parent) parent.children.push(node);
		else roots.push(node);
	}

	const sections = new Map<string, DocSection>();

	for (const root of roots) {
		const key = sectionKey(root.doc.audience);
		const section = sections.get(key) ?? {
			key,
			title: sectionTitle(root.doc.audience, projects),
			visibility: root.doc.audience.visibility,
			projectId: root.doc.audience.projectId,
			nodes: [],
		};

		section.nodes.push(root);
		sections.set(key, section);
	}

	const rank = (section: DocSection) => docVisibilities.indexOf(section.visibility);

	return {
		kind: "ready",
		docs,
		partial,
		sections: [...sections.values()]
			.sort((left, right) => rank(left) - rank(right) || titleOrder.compare(left.title, right.title))
			.map((section) => ({ ...section, nodes: sorted(section.nodes) })),
	};
}

function sectionTitle(audience: DocAudience, projects: Project[]): string {
	switch (audience.visibility) {
		case "workspace":
			return "Workspace";
		case "personal":
			return "Personal";
		case "project":
			return projects.find((project) => project.id === audience.projectId)?.name ?? "A project";
	}
}

export function projectName(audience: DocAudience, projects: Project[]): string | undefined {
	if (audience.visibility !== "project") return undefined;

	return projects.find((project) => project.id === audience.projectId)?.name;
}

export function ancestry(docs: DocSummary[], docId: string): DocSummary[] {
	const byId = new Map(docs.map((doc) => [doc.id, doc]));
	const trail: DocSummary[] = [];
	const visited = new Set<string>();

	let current = byId.get(docId)?.parentId;

	while (current && !visited.has(current)) {
		visited.add(current);

		const parent = byId.get(current);
		if (!parent) break;

		trail.unshift(parent);
		current = parent.parentId;
	}

	return trail;
}

export function descendants(docs: DocSummary[], docId: string): Set<string> {
	const found = new Set<string>();
	const pending = [docId];

	while (pending.length > 0) {
		const parentId = pending.pop();

		for (const doc of docs) {
			if (doc.parentId === parentId && !found.has(doc.id)) {
				found.add(doc.id);
				pending.push(doc.id);
			}
		}
	}

	return found;
}

export type DocReaders =
	| { kind: "author"; accountId?: string; name: string }
	| { kind: "people"; people: { accountId: string; name: string }[]; more: boolean }
	| { kind: "unavailable" };

export type DocLinks = { kind: "ready"; links: DocLink[] } | { kind: "unavailable" };

export type DocBacklinks = { kind: "ready"; docs: DocSummary[] } | { kind: "unavailable" };

export type DocDetail =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			doc: Doc;
			latest?: DocRevisionSummary;
			agents: Agent[];
			readers: DocReaders;
			links: DocLinks;
			candidates: IssueCandidate[];
	  };

export function unlinkedIssues(links: DocLink[], candidates: IssueCandidate[]): IssueCandidate[] {
	const linked = new Set(links.flatMap((link) => (link.issue ? [link.issue.id] : [])));

	return candidates.filter((candidate) => !linked.has(candidate.id));
}

export function unlinkedProjects(links: DocLink[], projects: Project[]): Project[] {
	const linked = new Set(links.flatMap((link) => (link.project ? [link.project.id] : [])));

	return projects.filter((project) => !linked.has(project.id));
}

export type DocEditing =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| { kind: "ready"; doc: Doc };

export type DocChosen = { revision: DocRevision; before?: DocRevision } | { kind: "unavailable" };

export type DocHistory =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			doc: Doc;
			revisions: DocRevisionSummary[];
			nextCursor?: string;
			agents: Agent[];
			chosen?: DocChosen;
	  };

export type DocFailure =
	| { kind: "stale"; version?: number }
	| { kind: "archived" }
	| { kind: "has_children" }
	| { kind: "cycle" }
	| { kind: "too_deep" }
	| { kind: "link_exists" }
	| { kind: "not_editable" }
	| { kind: "not_shareable" }
	| { kind: "not_found" }
	| { kind: "invalid"; fields: string[] }
	| { kind: "unavailable" };

const conflictKinds: Record<string, DocFailure["kind"]> = {
	doc_stale: "stale",
	doc_archived: "archived",
	doc_has_children: "has_children",
	doc_parent_cycle: "cycle",
	doc_tree_too_deep: "too_deep",
	doc_link_exists: "link_exists",
};

const forbiddenKinds: Record<string, DocFailure["kind"]> = {
	knowledge_not_editable: "not_editable",
	knowledge_not_shareable: "not_shareable",
};

export function readDocFailure(problem: unknown): DocFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	const read = problem as {
		code?: string;
		reason?: string;
		status?: number;
		version?: number;
		errors?: { field?: string }[];
	};

	const conflict = read.code ? conflictKinds[read.code] : undefined;

	if (conflict === "stale") return { kind: "stale", version: read.version };
	if (conflict) return { kind: conflict } as DocFailure;

	const forbidden = read.reason ? forbiddenKinds[read.reason] : undefined;

	if (forbidden) return { kind: forbidden } as DocFailure;
	if (read.status === 403) return { kind: "not_editable" };
	if (read.status === 404) return { kind: "not_found" };

	if (read.status === 422) {
		return {
			kind: "invalid",
			fields: (read.errors ?? []).flatMap((error) => (error.field ? [error.field] : [])),
		};
	}

	return { kind: "unavailable" };
}

const failureMessages: Record<DocFailure["kind"], string> = {
	stale: "Somebody else changed this doc while you had it open. Nothing you did was lost — reload to see where it stands.",
	archived: "That doc is archived, so nothing can change it or be filed under it.",
	has_children: "Docs are still filed under this one. Move or archive them first.",
	cycle: "A doc cannot be filed under itself, or under anything already beneath it.",
	too_deep: "That would nest docs more than ten levels deep. Pick somewhere shallower.",
	link_exists: "This doc already links there.",
	not_editable: "You can read this doc, but only its author, its contributors, and its maintainers can change it.",
	not_shareable: "You cannot share this doc with that audience. Only its author or a maintainer can change who reads it.",
	not_found: "That doc is gone, or it is not shared with you.",
	invalid: "Check the highlighted fields and try again.",
	unavailable: "Something went wrong and nothing changed. Wait a moment and try again.",
};

export function docFailureMessage(failure: DocFailure): string {
	return failureMessages[failure.kind];
}

export const revisionActionLabels: Record<DocRevisionAction, string> = {
	created: "Started the doc",
	edited: "Edited",
	moved: "Moved it",
	reshared: "Changed who reads it",
	restored: "Put back an earlier version",
	archived: "Archived it",
};

export const revisionSourceLabels: Record<DocRevisionSource, string> = {
	person: "written by hand",
	agent: "written by an agent",
	import: "brought in by an import",
	intake: "arrived by email",
	restore: "put back from the history",
};

export function revisionAgent(revision: DocRevisionSummary, agents: Agent[]): Agent | undefined {
	if (revision.source !== "agent" || !revision.agentId) return undefined;

	return agents.find((agent) => agent.id === revision.agentId);
}

export function revisionAuthor(revision: DocRevisionSummary, agents: Agent[]): string {
	const agent = revisionAgent(revision, agents);

	if (agent) return revision.accountName ? `${agent.name} for ${revision.accountName}` : agent.name;
	if (revision.source === "agent") return revision.accountName ? `An agent for ${revision.accountName}` : "An agent";

	return revision.accountName ?? "Somebody";
}

export function revisionLine(revision: DocRevisionSummary, agents: Agent[]): string {
	return `${revisionAuthor(revision, agents)} · ${revisionSourceLabels[revision.source]}`;
}

export function audienceLine(audience: DocAudience, projects: Project[], workspace: string): string {
	switch (audience.visibility) {
		case "workspace":
			return `Everybody in ${workspace} can read it.`;
		case "project":
			return `The members of ${projectName(audience, projects) ?? "its project"} can read it.`;
		case "personal":
			return "Only its author can read it.";
	}
}

export function sameAudience(left: DocAudience, right: DocAudience): boolean {
	return left.visibility === right.visibility && (left.projectId ?? "") === (right.projectId ?? "");
}
