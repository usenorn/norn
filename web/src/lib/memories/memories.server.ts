import type { Client } from "openapi-fetch";
import type { paths } from "#lib/api/dashboard.gen.js";
import type { Project } from "#lib/projects/projects.js";
import {
	memoryDraftOf,
	memoryListParams,
	memoryPageSize,
	memoryRevisionPageSize,
	projectMemoryCount,
	type MemoryChosen,
	type MemoryDetail,
	type MemoryDraft,
	type MemoryHistory,
	type MemoryList,
	type MemoryQuery,
	type ProjectMemories,
} from "./memories";

const recentRevisions = 3;

export async function listMemories(
	api: Client<paths>,
	workspaceId: string,
	query: MemoryQuery,
	cursor?: string
): Promise<MemoryList> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/memories", {
		params: {
			path: { workspaceId },
			query: { limit: memoryPageSize, cursor, ...memoryListParams(query) },
		},
	});

	if (error || !data) return { kind: "unavailable" };
	if (data.memories.length === 0 && !cursor) return { kind: "empty" };

	return { kind: "ready", memories: data.memories, nextCursor: data.nextCursor };
}

export async function readMemory(api: Client<paths>, workspaceId: string, memoryId: string): Promise<MemoryDetail> {
	const path = { workspaceId, memoryId };
	const [written, lineage, history] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/memories/{memoryId}/related", { params: { path } }),
		api.GET("/workspaces/{workspaceId}/memories/{memoryId}/lineage", { params: { path } }),
		api.GET("/workspaces/{workspaceId}/memories/{memoryId}/revisions", {
			params: { path, query: { limit: recentRevisions } },
		}),
	]);

	if (written.response.status === 404 || written.response.status === 400) return { kind: "not_found" };
	if (written.error || !written.data || !lineage.data || !history.data) return { kind: "unavailable" };

	return {
		kind: "ready",
		memory: written.data.memory,
		related: written.data.related,
		choices: written.data.choices,
		lineage: lineage.data.memories,
		recent: history.data.revisions,
	};
}

export async function readMemoryDraft(
	api: Client<paths>,
	workspaceId: string,
	memoryId: string,
	supersedes: boolean
): Promise<MemoryDraft> {
	const { data, error, response } = await api.GET("/workspaces/{workspaceId}/memories/{memoryId}", {
		params: { path: { workspaceId, memoryId } },
	});

	if (response.status === 404 || response.status === 400) return { kind: "not_found" };
	if (error || !data) return { kind: "unavailable" };

	return memoryDraftOf(data, supersedes);
}

export async function readMemoryHistory(
	api: Client<paths>,
	workspaceId: string,
	memoryId: string,
	revisionId?: string,
	cursor?: string
): Promise<MemoryHistory> {
	const path = { workspaceId, memoryId };
	const [read, listed] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/memories/{memoryId}", { params: { path } }),
		api.GET("/workspaces/{workspaceId}/memories/{memoryId}/revisions", {
			params: { path, query: { limit: memoryRevisionPageSize, cursor } },
		}),
	]);

	if (read.response.status === 404 || read.response.status === 400) return { kind: "not_found" };
	if (read.error || !read.data || listed.error || !listed.data) return { kind: "unavailable" };

	const { revisions, nextCursor } = listed.data;
	const summary = revisions.find((revision) => revision.id === revisionId) ?? revisions[0];

	return {
		kind: "ready",
		memory: read.data,
		revisions,
		nextCursor,
		chosen: summary ? await readRevision(api, path, summary.id) : undefined,
	};
}

async function readRevision(
	api: Client<paths>,
	path: { workspaceId: string; memoryId: string },
	revisionId: string
): Promise<MemoryChosen> {
	const { data } = await api.GET("/workspaces/{workspaceId}/memories/{memoryId}/revisions/{revisionId}", {
		params: { path: { ...path, revisionId } },
	});

	return data ? { kind: "ready", revision: data } : { kind: "unavailable" };
}

export async function readProjectMemories(
	api: Client<paths>,
	workspaceId: string,
	projectId: string
): Promise<ProjectMemories> {
	const path = { workspaceId };
	const [accepted, awaiting] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/memories", {
			params: { path, query: { projectId, status: ["accepted"], limit: projectMemoryCount } },
		}),
		api.GET("/workspaces/{workspaceId}/memories", {
			params: { path, query: { projectId, reviewable: true, limit: 1 } },
		}),
	]);

	if (!accepted.data || !awaiting.data) return { kind: "unavailable" };

	return {
		kind: "ready",
		memories: accepted.data.memories,
		more: Boolean(accepted.data.nextCursor),
		awaiting: awaiting.data.memories.length > 0 || Boolean(awaiting.data.nextCursor),
	};
}

export async function readProjects(api: Client<paths>, workspaceId: string): Promise<Project[]> {
	const { data } = await api.GET("/workspaces/{workspaceId}/projects", { params: { path: { workspaceId } } });

	return data ?? [];
}
