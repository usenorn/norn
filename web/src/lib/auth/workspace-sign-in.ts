import type { Client } from "openapi-fetch";
import type { components, paths } from "$lib/api/dashboard.gen";

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
	api: Client<paths>,
	requested: string
): Promise<WorkspaceEntry> {
	const workspace = workspaceSlug(requested);

	if (!workspace) return { kind: "none" };

	try {
		const { data } = await api.GET("/sso/sign-in", {
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
