import type { components } from "#lib/api/dashboard.gen.js";
import type { PillTone } from "#lib/components/norn/pill.svelte";
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

export const memoryPageSize = 50;

export type MemoryFilter = "review" | "accepted" | "all";

export const memoryFilters: MemoryFilter[] = ["review", "accepted", "all"];

export const memoryFilterLabels: Record<MemoryFilter, string> = {
	review: "Needs review",
	accepted: "Accepted",
	all: "All",
};

const filterStatuses: Record<MemoryFilter, MemoryStatus[]> = {
	review: ["proposed", "disputed"],
	accepted: ["accepted"],
	all: [],
};

export function memoryFilterOf(raw: string | null): MemoryFilter {
	return memoryFilters.find((filter) => filter === raw) ?? "review";
}

export function memoryFilterStatuses(filter: MemoryFilter): MemoryStatus[] {
	return filterStatuses[filter];
}

export function memoriesPath(workspace: string, filter: MemoryFilter = "review", cursor?: string): string {
	const query = new URLSearchParams();

	if (filter !== "review") query.set("filter", filter);
	if (cursor) query.set("cursor", cursor);

	const search = query.toString();

	return workspacePath(workspace, search ? `/memories?${search}` : "/memories");
}

export function memoryPath(workspace: string, memoryId: string): string {
	return workspacePath(workspace, `/memories/${memoryId}`);
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
	| { kind: "ready"; memory: Memory; related: RelatedMemory[]; choices: MemoryChoice[] };

export const memoryStatusLabels: Record<MemoryStatus, string> = {
	proposed: "Proposed",
	accepted: "Accepted",
	disputed: "Disputed",
	superseded: "Superseded",
	withdrawn: "Withdrawn",
};

export const memoryStatusTones: Record<MemoryStatus, PillTone> = {
	proposed: "active",
	accepted: "success",
	disputed: "danger",
	superseded: "muted",
	withdrawn: "muted",
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

export function scopeLine(memory: Memory, projects: Project[]): string {
	const project = memory.appliesTo.projectId
		? (projects.find((candidate) => candidate.id === memory.appliesTo.projectId)?.name ?? "One project")
		: "The whole workspace";

	return memory.appliesTo.component ? `${project} · ${memory.appliesTo.component}` : project;
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
	not_found: "That memory is gone, or it is not shared with you.",
	invalid: "That choice no longer fits these memories. Reload and pick again.",
	unavailable: "Something went wrong and nothing changed. Wait a moment and try again.",
};

export function memoryFailureMessage(failure: MemoryFailure): string {
	return failureMessages[failure.kind];
}
