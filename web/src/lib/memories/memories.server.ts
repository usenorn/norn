import type { Client } from "openapi-fetch";
import type { paths } from "#lib/api/dashboard.gen.js";
import type { Project } from "#lib/projects/projects.js";
import { memoryFilterStatuses, memoryPageSize, type MemoryDetail, type MemoryFilter, type MemoryList } from "./memories";

export async function listMemories(
	api: Client<paths>,
	workspaceId: string,
	filter: MemoryFilter,
	cursor?: string
): Promise<MemoryList> {
	const statuses = memoryFilterStatuses(filter);
	const { data, error } = await api.GET("/workspaces/{workspaceId}/memories", {
		params: {
			path: { workspaceId },
			query: { limit: memoryPageSize, cursor, ...(statuses.length > 0 ? { status: statuses } : {}) },
		},
	});

	if (error || !data) return { kind: "unavailable" };
	if (data.memories.length === 0) return { kind: "empty" };

	return { kind: "ready", memories: data.memories, nextCursor: data.nextCursor };
}

export async function readMemory(api: Client<paths>, workspaceId: string, memoryId: string): Promise<MemoryDetail> {
	const { data, error, response } = await api.GET("/workspaces/{workspaceId}/memories/{memoryId}/related", {
		params: { path: { workspaceId, memoryId } },
	});

	if (response.status === 404 || response.status === 400) return { kind: "not_found" };
	if (error || !data) return { kind: "unavailable" };

	return { kind: "ready", memory: data.memory, related: data.related, choices: data.choices };
}

export async function readProjects(api: Client<paths>, workspaceId: string): Promise<Project[]> {
	const { data } = await api.GET("/workspaces/{workspaceId}/projects", { params: { path: { workspaceId } } });

	return data ?? [];
}
