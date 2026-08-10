import { keys } from "$lib/api/keys";
import { actingOf, type ActingSession } from "$lib/account/accounts";
import type { LayoutServerLoad } from "./$types";

export const load: LayoutServerLoad = async ({ depends, locals, params, url }) => {
	depends(keys.signedIn());

	const accounts = await locals.signedIn;
	const acting: ActingSession | undefined = actingOf(accounts, url, params.workspace);

	return { accounts, acting };
};
