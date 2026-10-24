import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import type { Agent } from "$lib/agents/agents";
import {
	docReadersShown,
	docTreePageSize,
	docTreePagesMax,
	type Doc,
	type DocAudience,
	type DocReaders,
	type DocSummary,
} from "./docs";

export type DocRead = { kind: "ready"; doc: Doc } | { kind: "not_found" } | { kind: "unavailable" };

export async function readDoc(api: Client<paths>, workspaceId: string, docId: string): Promise<DocRead> {
	const { data, error, response } = await api.GET("/workspaces/{workspaceId}/docs/{docId}", {
		params: { path: { workspaceId, docId } },
	});

	if (response.status === 404 || response.status === 400) return { kind: "not_found" };
	if (error || !data) return { kind: "unavailable" };

	return { kind: "ready", doc: data };
}

export type DocListing = { docs: DocSummary[]; partial: boolean } | null;

export async function listDocs(api: Client<paths>, workspaceId: string): Promise<DocListing> {
	const docs: DocSummary[] = [];
	let cursor: string | undefined;

	for (let page = 0; page < docTreePagesMax; page++) {
		const { data, error } = await api.GET("/workspaces/{workspaceId}/docs", {
			params: { path: { workspaceId }, query: { limit: docTreePageSize, cursor } },
		});

		if (error || !data) return null;

		docs.push(...data.docs);
		cursor = data.nextCursor;

		if (!cursor) return { docs, partial: false };
	}

	return { docs, partial: true };
}

export async function readAgents(api: Client<paths>, workspaceId: string): Promise<Agent[]> {
	const { data } = await api.GET("/workspaces/{workspaceId}/agents", {
		params: { path: { workspaceId } },
	});

	return (data ?? []).map((listed) => listed.agent);
}

export async function readReaders(
	api: Client<paths>,
	workspaceId: string,
	audience: DocAudience,
	author: { accountId?: string; name?: string }
): Promise<DocReaders> {
	switch (audience.visibility) {
		case "personal":
			return { kind: "author", accountId: author.accountId, name: author.name ?? "Its author" };
		case "workspace": {
			const { data } = await api.GET("/workspaces/{workspaceId}/members", {
				params: { path: { workspaceId }, query: { kind: ["person"], limit: docReadersShown } },
			});

			if (!data) return { kind: "unavailable" };

			return {
				kind: "people",
				people: data.members.map((member) => ({
					accountId: member.accountId,
					name: member.displayName || member.email || "A member",
				})),
				more: Boolean(data.nextCursor),
			};
		}
		case "project": {
			if (!audience.projectId) return { kind: "unavailable" };

			const { data } = await api.GET("/workspaces/{workspaceId}/projects/{projectId}/members", {
				params: { path: { workspaceId, projectId: audience.projectId } },
			});

			if (!data) return { kind: "unavailable" };

			return {
				kind: "people",
				people: data
					.slice(0, docReadersShown)
					.map((member) => ({ accountId: member.accountId, name: member.displayName || member.email })),
				more: data.length > docReadersShown,
			};
		}
	}
}
