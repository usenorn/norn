import { redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import type { LayoutLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type AccountScope = {
	now: string;
	workspaces: WorkspaceSummary[];
	account: { id: string; name: string };
};

export const load: LayoutLoad = async ({ fetch, url }): Promise<AccountScope> => {
	const api = apiFor(url);

	const [workspaces, account] = await Promise.all([
		api.GET("/workspaces", { fetch }),
		api.GET("/accounts/me", { fetch }),
	]);

	if (workspaces.error || !workspaces.data) {
		redirect(307, `/sign-in?return=${encodeURIComponent(url.pathname + url.search)}`);
	}

	return {
		now: new Date().toISOString(),
		workspaces: workspaces.data,
		account: { id: account.data?.id ?? "", name: account.data?.displayName ?? "" },
	};
};
