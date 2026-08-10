import type { RequestEvent } from "@sveltejs/kit";
import { reachOfSlug, sessionParam, type SignedInAccount } from "$lib/account/accounts";
import { apiForEvent, hasSession } from "./server";

// The directory is read through an unselected client, because it answers for every cookie on the
// request rather than for one of them. It is stored unawaited: awaiting it in `handle` would put
// a round trip in front of every page, including the ones nobody is signed in for.
export function signedInAccounts(event: RequestEvent): Promise<SignedInAccount[]> {
	if (!hasSession(event.cookies)) return Promise.resolve([]);

	return apiForEvent(event)
		.GET("/accounts/signed-in")
		.then(({ data }) => data ?? [])
		.catch(() => []);
}

// A workspace address decides which session acts in it, so a selector can only choose between
// sessions that reach that workspace — never override it. Anywhere else the selector is the only
// thing that names a session, and with one signed in it names itself.
export function actingSlot(event: RequestEvent): Promise<string | null> {
	if (!hasSession(event.cookies)) return Promise.resolve(null);

	const named = event.url.searchParams.get(sessionParam);
	const slug = event.params.workspace;

	if (!slug) return Promise.resolve(named);

	return event.locals.signedIn.then((accounts) => {
		const reach = reachOfSlug(accounts, slug, named);

		return reach?.workspace.slot ?? named;
	});
}
