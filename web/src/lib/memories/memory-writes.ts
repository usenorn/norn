import { invalidate } from "$app/navigation";
import { api } from "#lib/api/index.js";
import { attempt, type Outcome } from "#lib/api/attempt.js";
import { keys } from "#lib/api/keys.js";
import type { DocAudience } from "#lib/docs/docs.js";
import type { Memory } from "./memories";

export function acceptMemory(workspaceId: string, memory: Memory): Promise<Outcome<Memory>> {
	return attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/memories/{memoryId}/accept", {
				params: { path: { workspaceId, memoryId: memory.id } },
				body: { expectedVersion: memory.version },
			}),
	});
}

export function rejectMemory(workspaceId: string, memory: Memory, reason: string): Promise<Outcome<Memory>> {
	return attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/memories/{memoryId}/reject", {
				params: { path: { workspaceId, memoryId: memory.id } },
				body: { expectedVersion: memory.version, reason },
			}),
	});
}

export function withdrawMemory(workspaceId: string, memory: Memory): Promise<Outcome<Memory>> {
	return attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/memories/{memoryId}/withdraw", {
				params: { path: { workspaceId, memoryId: memory.id } },
				body: { expectedVersion: memory.version },
			}),
	});
}

export function shareMemory(workspaceId: string, memory: Memory, audience: DocAudience): Promise<Outcome<Memory>> {
	return attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/memories/{memoryId}/share", {
				params: { path: { workspaceId, memoryId: memory.id } },
				body: { expectedVersion: memory.version, audience },
			}),
	});
}

export function refreshMemory(workspaceId: string, memoryId: string): Promise<unknown> {
	return Promise.all([invalidate(keys.memory(memoryId)), invalidate(keys.memories(workspaceId))]);
}
