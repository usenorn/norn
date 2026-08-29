import { redirect } from "@sveltejs/kit";
import { withSlot } from "$lib/account/accounts";
import { lastWorkspaceCookie, rememberedWorkspace } from "$lib/account/last-workspace";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { accounts, acting } = await parent();

	if (!acting) redirect(307, "/sign-in");

	const signedIn =
		accounts.find((candidate) => candidate.account.id === acting.accountId) ?? accounts[0];

	const reached = signedIn?.workspaces ?? [];
	const remembered = signedIn
		? rememberedWorkspace(reached, cookies.get(lastWorkspaceCookie(signedIn.account.id)))
		: undefined;

	const landing = remembered ?? reached[0];

	if (!landing) redirect(307, withSlot("/create-workspace", acting.slot));

	redirect(307, withSlot(`/${landing.workspace.slug}`, landing.slot));
};
