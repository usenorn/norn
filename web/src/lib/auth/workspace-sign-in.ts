import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";

export type WorkspaceSignIn = components["schemas"]["WorkspaceSignIn"];

export type WorkspaceEntry =
	| { kind: "none" }
	| { kind: "unknown"; workspace: string }
	| { kind: "ready"; signIn: WorkspaceSignIn };

export function workspaceSlug(raw: string): string {
	const trimmed = raw.trim().toLowerCase();
	const tail = trimmed.includes("/") ? (trimmed.split("/").pop() ?? "") : trimmed;

	return tail.replace(/[^a-z0-9-]/g, "");
}

export async function reachWorkspaceSignIn(
	fetch: typeof globalThis.fetch,
	url: URL,
	requested: string
): Promise<WorkspaceEntry> {
	const workspace = workspaceSlug(requested);

	if (!workspace) return { kind: "none" };

	const api = apiFor(url);

	try {
		const { data } = await api.GET("/sso/sign-in", {
			fetch,
			params: { query: { workspace } },
		});

		return data ? { kind: "ready", signIn: data } : { kind: "unknown", workspace };
	} catch {
		return { kind: "unknown", workspace };
	}
}

export function ssoEntryPoint(workspace: string, returnTo: string): string {
	const query = new URLSearchParams({ workspace });

	if (returnTo && returnTo !== "/") query.set("return_to", returnTo);

	return `/sso?${query.toString()}`;
}
