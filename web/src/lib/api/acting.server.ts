import type { RequestEvent } from "@sveltejs/kit";
import { actingOf, type SignedInAccount } from "$lib/account/accounts";
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

// The header this resolves and the identity the page renders have to be the same session, or a
// slot the directory does not know is sent as a selector — which the API answers by authenticating
// nobody — while the screen reports the account it fell back to, and a refusal reads as a
// permission the person does not have.
export function actingSlot(event: RequestEvent): Promise<string | null> {
	if (!hasSession(event.cookies)) return Promise.resolve(null);

	return event.locals.signedIn.then(
		(accounts) => actingOf(accounts, event.url, event.params.workspace)?.slot ?? null
	);
}
