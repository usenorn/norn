import { keys } from "$lib/api/keys";
import { accountOfSlot, defaultSlot, type ActingSession } from "$lib/account/accounts";
import type { LayoutServerLoad } from "./$types";

// Reads only what `handle` resolved, never the url or the params: anything tracked here would
// re-run this load on every navigation and put a directory round trip in front of each one.
export const load: LayoutServerLoad = async ({ depends, locals }) => {
	depends(keys.signedIn());

	const accounts = await locals.signedIn;
	const slot = (await locals.acting) ?? defaultSlot(accounts);
	const account = accountOfSlot(accounts, slot);

	const acting: ActingSession | undefined =
		slot && account ? { slot, accountId: account.account.id } : undefined;

	return { accounts, acting };
};
