import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import type { SignedInAccount } from "$lib/account/accounts";
import type { components } from "$lib/api/dashboard.gen";
import type { LayoutServerLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type AccountScope = {
	now: string;
	accounts: SignedInAccount[];
	workspaces: WorkspaceSummary[];
	account: { id: string; name: string; email: string; slot: string };
};

export const load: LayoutServerLoad = async ({ depends, parent, url }): Promise<AccountScope> => {
	const { accounts, acting } = await parent();

	const signedIn = acting
		? accounts.find((candidate) => candidate.account.id === acting.accountId)
		: undefined;

	if (!acting || !signedIn) {
		redirect(307, `/sign-in?return=${encodeURIComponent(url.pathname + url.search)}`);
	}

	depends(keys.account(signedIn.account.id));

	return {
		now: new Date().toISOString(),
		accounts,
		workspaces: signedIn.workspaces.map((reach) => reach.workspace),
		account: {
			id: signedIn.account.id,
			name: signedIn.account.displayName,
			email: signedIn.account.email,
			slot: acting.slot,
		},
	};
};
