import { redirect } from "@sveltejs/kit";
import { withSlot } from "$lib/account/accounts";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ parent }) => {
	const { accounts, acting } = await parent();

	if (!acting) redirect(307, "/sign-in");

	const signedIn =
		accounts.find((candidate) => candidate.account.id === acting.accountId) ?? accounts[0];

	const [first] = signedIn?.workspaces ?? [];

	if (!first) redirect(307, withSlot("/create-workspace", acting.slot));

	redirect(307, `/${first.workspace.slug}`);
};
