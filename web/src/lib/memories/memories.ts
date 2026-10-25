import type { components } from "#lib/api/dashboard.gen.js";
import type { PillTone } from "#lib/components/norn/pill.svelte";
import { docVisibilities, docVisibilityLabels, type DocVisibility } from "#lib/docs/docs.js";
import type { Project } from "#lib/projects/projects.js";
import { workspacePath } from "#lib/workspace/navigation.js";

export type Memory = components["schemas"]["Memory"];
export type MemoryKind = components["schemas"]["MemoryKind"];
export type MemoryStatus = components["schemas"]["MemoryStatus"];
export type MemoryBasis = components["schemas"]["MemoryBasis"];
export type MemoryRelation = components["schemas"]["MemoryRelation"];
export type MemoryResolution = components["schemas"]["MemoryResolution"];
export type MemoryChoice = components["schemas"]["MemoryChoice"];
export type RelatedMemory = components["schemas"]["RelatedMemory"];

export type MemoryRevisionSummary = components["schemas"]["MemoryRevisionSummary"];
export type MemoryRevision = components["schemas"]["MemoryRevision"];
export type MemoryRevisionAction = components["schemas"]["MemoryRevisionAction"];
export type MemoryEvidence = components["schemas"]["MemoryEvidence"];
export type MemoryContent = components["schemas"]["MemoryContent"];

export const memoryPageSize = 50;
export const memoryRevisionPageSize = 30;
export const memoryStatementMax = 500;
export const memoryProseMax = 4000;
export const memoryComponentMax = 100;
export const memoryReasonMax = 1000;
export const wholeWorkspace = "workspace";

export type MemoryTab = "accepted" | "review" | "all";

export const memoryTabs: MemoryTab[] = ["accepted", "review", "all"];

export const memoryTabLabels: Record<MemoryTab, string> = {
	accepted: "Accepted",
	review: "For your review",
	all: "All",
};

export type MemoryFacet = "scope" | "kind" | "status" | "project" | "component";

export const memoryFacetLabels: Record<MemoryFacet, string> = {
	scope: "Shared with",
	kind: "Kind",
	status: "Status",
	project: "Applies to",
	component: "Component",
};

export type MemoryQuery = {
	tab: MemoryTab;
	scope?: DocVisibility;
	kind?: MemoryKind;
	status?: MemoryStatus;
	project?: string;
	component?: string;
};

export const memoryKinds: MemoryKind[] = ["decision", "constraint", "lesson", "preference"];

export const memoryStatuses: MemoryStatus[] = ["proposed", "accepted", "disputed", "superseded", "withdrawn", "rejected"];

export function memoryFacetsFor(tab: MemoryTab): MemoryFacet[] {
	return tab === "all" ? ["scope", "kind", "status", "project", "component"] : ["scope", "kind", "project", "component"];
}

function oneOf<T extends string>(options: readonly T[], raw: string | null): T | undefined {
	return options.find((option) => option === raw);
}

export function memoryQueryOf(search: URLSearchParams): MemoryQuery {
	const tab = oneOf(memoryTabs, search.get("tab")) ?? "accepted";
	const component = search.get("component")?.trim();

	return {
		tab,
		scope: oneOf(docVisibilities, search.get("scope")),
		kind: oneOf(memoryKinds, search.get("kind")),
		status: tab === "all" ? oneOf(memoryStatuses, search.get("status")) : undefined,
		project: search.get("project") || undefined,
		component: component || undefined,
	};
}

export function memoryQueryFiltered(query: MemoryQuery): boolean {
	return memoryFacetsFor(query.tab).some((facet) => query[facet] !== undefined);
}

export function memoriesPath(workspace: string, query: Partial<MemoryQuery> = {}, cursor?: string): string {
	const search = new URLSearchParams();
	const tab = query.tab ?? "accepted";

	if (tab !== "accepted") search.set("tab", tab);

	for (const facet of memoryFacetsFor(tab)) {
		const value = query[facet];
		if (value) search.set(facet, value);
	}

	if (cursor) search.set("cursor", cursor);

	const encoded = search.toString();

	return workspacePath(workspace, encoded ? `/memories?${encoded}` : "/memories");
}

export type MemoryFacetValue = { value: string; label: string };

export function memoryFacetValues(facet: MemoryFacet, projects: Project[], components: string[]): MemoryFacetValue[] {
	switch (facet) {
		case "scope":
			return docVisibilities.map((value) => ({ value, label: docVisibilityLabels[value] }));
		case "kind":
			return memoryKinds.map((value) => ({ value, label: memoryKindLabels[value] }));
		case "status":
			return memoryStatuses.map((value) => ({ value, label: memoryStatusLabels[value] }));
		case "project":
			return projects.map((project) => ({ value: project.id, label: project.name }));
		case "component":
			return [...new Set(components)].sort().map((value) => ({ value, label: value }));
	}
}

export function memoryFacetValueLabel(facet: MemoryFacet, value: string, projects: Project[]): string {
	return memoryFacetValues(facet, projects, [value]).find((option) => option.value === value)?.label ?? value;
}

export type MemoryFacetChip = { facet: MemoryFacet; label: string; removed: string };

export function memoryFacetChips(workspace: string, query: MemoryQuery, projects: Project[]): MemoryFacetChip[] {
	return memoryFacetsFor(query.tab).flatMap((facet) => {
		const value = query[facet];

		if (!value) return [];

		return [
			{
				facet,
				label: `${memoryFacetLabels[facet]}: ${memoryFacetValueLabel(facet, value, projects)}`,
				removed: memoriesPath(workspace, { ...query, [facet]: undefined }),
			},
		];
	});
}

export type MemoryListParams = {
	reviewable?: boolean;
	status?: MemoryStatus[];
	visibility?: DocVisibility[];
	kind?: MemoryKind[];
	projectId?: string;
	component?: string;
};

export function memoryListParams(query: MemoryQuery): MemoryListParams {
	const params: MemoryListParams = {};

	if (query.tab === "review") params.reviewable = true;
	if (query.tab === "accepted") params.status = ["accepted"];
	if (query.tab === "all" && query.status) params.status = [query.status];
	if (query.scope) params.visibility = [query.scope];
	if (query.kind) params.kind = [query.kind];
	if (query.project) params.projectId = query.project;
	if (query.component) params.component = query.component;

	return params;
}

export function memoryPath(workspace: string, memoryId: string): string {
	return workspacePath(workspace, `/memories/${memoryId}`);
}

export function memoryEditPath(workspace: string, memoryId: string): string {
	return `${memoryPath(workspace, memoryId)}/edit`;
}

export function memorySupersedePath(workspace: string, memoryId: string): string {
	return `${memoryPath(workspace, memoryId)}/supersede`;
}

export function memoryHistoryPath(workspace: string, memoryId: string, revisionId?: string, cursor?: string): string {
	const search = new URLSearchParams();

	if (cursor) search.set("cursor", cursor);
	if (revisionId) search.set("revision", revisionId);

	const encoded = search.toString();
	const path = `${memoryPath(workspace, memoryId)}/history`;

	return encoded ? `${path}?${encoded}` : path;
}

export type MemoryList =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "empty" }
	| { kind: "ready"; memories: Memory[]; nextCursor?: string };

export type MemoryDetail =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			memory: Memory;
			related: RelatedMemory[];
			choices: MemoryChoice[];
			lineage: Memory[];
			recent: MemoryRevisionSummary[];
	  };

export type MemoryDraft =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| { kind: "read_only"; memory: Memory }
	| { kind: "closed"; memory: Memory }
	| { kind: "ready"; memory: Memory };

export function memoryDraftOf(memory: Memory, supersedes: boolean): MemoryDraft {
	if (memoryClosed(memory) || (supersedes && memory.status === "proposed")) return { kind: "closed", memory };
	if (!memory.viewer.canEdit) return { kind: "read_only", memory };

	return { kind: "ready", memory };
}

export type MemoryChosen = { kind: "ready"; revision: MemoryRevision } | { kind: "unavailable" };

export type MemoryHistory =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			memory: Memory;
			revisions: MemoryRevisionSummary[];
			nextCursor?: string;
			chosen?: MemoryChosen;
	  };

export const memoryStatusLabels: Record<MemoryStatus, string> = {
	proposed: "Proposed",
	accepted: "Accepted",
	disputed: "Disputed",
	superseded: "Superseded",
	withdrawn: "Withdrawn",
	rejected: "Rejected",
};

export const memoryStatusTones: Record<MemoryStatus, PillTone> = {
	proposed: "active",
	accepted: "success",
	disputed: "danger",
	superseded: "muted",
	withdrawn: "muted",
	rejected: "muted",
};

export const memoryKindLabels: Record<MemoryKind, string> = {
	decision: "Decision",
	constraint: "Constraint",
	lesson: "Lesson",
	preference: "Preference",
};

export const memoryBasisLabels: Record<MemoryBasis, string> = {
	observation: "Observed",
	inference: "Inferred by an agent",
};

export const memoryRelationLabels: Record<MemoryRelation, string> = {
	duplicate: "Possible duplicate",
	conditional: "Same subject, other conditions",
	conflict: "Contradicts",
};

export const memoryRelationLines: Record<MemoryRelation, string> = {
	duplicate: "It covers the same subject under the same conditions.",
	conditional: "It covers the same subject under other conditions, so both can hold.",
	conflict: "It is an accepted decision that says otherwise under the same conditions.",
};

export const memoryRelationTones: Record<MemoryRelation, PillTone> = {
	duplicate: "active",
	conditional: "muted",
	conflict: "danger",
};

export type MemoryHold = "disputed" | "proposed" | "settled";

export function memoryHold(memory: Memory, related: RelatedMemory[]): MemoryHold {
	if (related.length === 0) return "settled";

	return memory.status === "disputed" ? "disputed" : memory.status === "proposed" ? "proposed" : "settled";
}

export function choicesFor(choices: MemoryChoice[], targetId: string): MemoryResolution[] {
	return choices.flatMap((choice) => (choice.targetId === targetId ? [choice.resolution] : []));
}

export function offers(choices: MemoryChoice[], resolution: MemoryResolution): boolean {
	return choices.some((choice) => choice.resolution === resolution && !choice.targetId);
}

export function scopeLine(memory: Pick<Memory, "appliesTo">, projects: Project[]): string {
	const project = memory.appliesTo.projectId
		? (projects.find((candidate) => candidate.id === memory.appliesTo.projectId)?.name ?? "One project")
		: "The whole workspace";

	return memory.appliesTo.component ? `${project} · ${memory.appliesTo.component}` : project;
}

export function sharedWithLine(audience: Memory["audience"], projects: Project[]): string {
	if (audience.visibility !== "project") return docVisibilityLabels[audience.visibility];

	return projects.find((project) => project.id === audience.projectId)?.name ?? "One project";
}

export function memoryAuthorLine(memory: Pick<Memory, "authorName" | "agentName" | "source">): string {
	if (memory.agentName) return memory.authorName ? `${memory.agentName} for ${memory.authorName}` : memory.agentName;
	if (memory.source === "agent") return memory.authorName ? `An agent for ${memory.authorName}` : "An agent";

	return memory.authorName ?? "Somebody";
}

export function revisionAuthorLine(revision: MemoryRevisionSummary): string {
	return memoryAuthorLine({ authorName: revision.accountName, agentName: revision.agentName, source: revision.source });
}

export const memoryRevisionActionLabels: Record<MemoryRevisionAction, string> = {
	created: "Recorded it",
	corrected: "Corrected it",
	reshared: "Changed who reads it",
	accepted: "Accepted it",
	disputed: "Held it as disputed",
	superseded: "Superseded it",
	withdrawn: "Withdrew it",
	rejected: "Rejected it",
	reconciled: "Settled how it relates",
};

export type EvidenceLink = { label: string; detail?: string; href?: string; quote?: string };

export function evidenceLink(workspace: string, evidence: MemoryEvidence): EvidenceLink {
	switch (evidence.kind) {
		case "issue":
		case "comment":
			return {
				label: evidence.issueTitle ?? "An issue",
				detail: evidence.kind === "comment" ? "A comment on an issue" : "An issue",
				href: evidence.issueReference ? workspacePath(workspace, `/issues/${evidence.issueReference}`) : undefined,
			};
		case "doc_revision":
			return {
				label: evidence.docTitle ?? "A doc",
				detail: "A version of a doc",
				href: evidence.docId
					? `${workspacePath(workspace, `/docs/${evidence.docId}/history`)}${evidence.docRevisionId ? `?revision=${evidence.docRevisionId}` : ""}`
					: undefined,
			};
		case "excerpt":
			return { label: "A pasted excerpt", quote: evidence.excerpt ?? "" };
	}
}

export function memoryClosed(memory: Memory): boolean {
	return memory.status === "superseded" || memory.status === "withdrawn" || memory.status === "rejected";
}

export function memoryPending(memory: Memory): boolean {
	return memory.status === "proposed" || memory.status === "disputed";
}

export type MemoryAction = "accept" | "reject" | "withdraw" | "share";

export type MemoryActivity =
	| { kind: "idle" }
	| { kind: "working"; action: MemoryAction }
	| { kind: "failed"; action: MemoryAction; failure: MemoryFailure };

export const memoryActionTitles: Record<MemoryAction, string> = {
	accept: "It was not accepted",
	reject: "It was not rejected",
	withdraw: "It was not withdrawn",
	share: "It was not shared differently",
};

export type MemoryContentFields = {
	kind: MemoryKind;
	statement: string;
	rationale: string;
	conditions: string;
	appliesTo: string;
	component: string;
	basis: MemoryBasis;
	effectiveFrom: string;
	effectiveUntil: string;
};

export function memoryContentFields(memory: Memory): MemoryContentFields {
	return {
		kind: memory.kind,
		statement: memory.statement,
		rationale: memory.rationale,
		conditions: memory.conditions,
		appliesTo: memory.appliesTo.projectId ?? wholeWorkspace,
		component: memory.appliesTo.component ?? "",
		basis: memory.basis,
		effectiveFrom: memory.effectiveFrom,
		effectiveUntil: memory.effectiveUntil ?? "",
	};
}

export function memoryContentOf(fields: MemoryContentFields, evidence: MemoryEvidence[] = []): MemoryContent {
	return {
		kind: fields.kind,
		statement: fields.statement,
		rationale: fields.rationale,
		conditions: fields.conditions,
		appliesTo: {
			...(fields.appliesTo !== wholeWorkspace ? { projectId: fields.appliesTo } : {}),
			...(fields.component ? { component: fields.component } : {}),
		},
		basis: fields.basis,
		evidence: evidence.map((cited) => ({
			kind: cited.kind,
			issueId: cited.issueId,
			commentId: cited.commentId,
			docId: cited.docId,
			docRevisionId: cited.docRevisionId,
			excerpt: cited.excerpt,
		})),
		...(fields.effectiveFrom ? { effectiveFrom: fields.effectiveFrom } : {}),
		...(fields.effectiveUntil ? { effectiveUntil: fields.effectiveUntil } : {}),
	};
}

export const reconciledLines: Record<MemoryResolution, string> = {
	independent: "Kept as its own memory",
	update: "Folded into the existing memory",
	supersede: "The earlier memory is superseded",
	no_change: "Dropped; nothing else changed",
};

export type MemoryFailure =
	| { kind: "stale"; version?: number }
	| { kind: "closed" }
	| { kind: "transition" }
	| { kind: "settled" }
	| { kind: "not_editable" }
	| { kind: "not_reviewer" }
	| { kind: "not_found" }
	| { kind: "invalid" }
	| { kind: "unavailable" };

const conflictKinds: Record<string, MemoryFailure["kind"]> = {
	memory_stale: "stale",
	memory_closed: "closed",
	memory_transition_invalid: "transition",
	memory_not_pending: "settled",
};

export function readMemoryFailure(problem: unknown): MemoryFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	const read = problem as { code?: string; reason?: string; status?: number; version?: number };
	const conflict = read.code ? conflictKinds[read.code] : undefined;

	if (conflict === "stale") return { kind: "stale", version: read.version };
	if (conflict) return { kind: conflict } as MemoryFailure;
	if (read.reason === "memory_review_forbidden") return { kind: "not_reviewer" };
	if (read.reason === "knowledge_not_editable" || read.status === 403) return { kind: "not_editable" };
	if (read.status === 404) return { kind: "not_found" };
	if (read.status === 422) return { kind: "invalid" };

	return { kind: "unavailable" };
}

const failureMessages: Record<MemoryFailure["kind"], string> = {
	stale: "One of these memories changed while you had it open. Reload to see where it stands.",
	closed: "That memory is already superseded or withdrawn, so it takes no more changes.",
	transition: "That memory cannot move that way from where it stands now.",
	settled: "This memory is already settled. Reload to see where it stands.",
	not_editable: "Only the author, its contributors, and its maintainers can change that memory.",
	not_reviewer: "Only a workspace admin, or the lead of its project, can accept or reject it.",
	not_found: "That memory is gone, or it is not shared with you.",
	invalid: "That choice no longer fits these memories. Reload and pick again.",
	unavailable: "Something went wrong and nothing changed. Wait a moment and try again.",
};

export function memoryFailureMessage(failure: MemoryFailure): string {
	return failureMessages[failure.kind];
}
