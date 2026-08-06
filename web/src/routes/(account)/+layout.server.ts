import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import type { components } from "$lib/api/dashboard.gen";
import type { LayoutServerLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type AccountScope = {
	now: string;
	workspaces: WorkspaceSummary[];
	account: { id: string; name: string };
};

export const load: LayoutServerLoad = async ({
	depends,
	locals,
	url,
}): Promise<AccountScope> => {
	depends(keys.account());

	const [workspaces, account] = await Promise.all([
		locals.api.GET("/workspaces"),
		locals.api.GET("/accounts/me"),
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
