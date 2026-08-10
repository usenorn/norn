import type { RequestEvent } from "@sveltejs/kit";
import { actingOf, type SignedInAccount } from "$lib/account/accounts";
import { apiForEvent, hasSession } from "./server";

export function signedInAccounts(event: RequestEvent): Promise<SignedInAccount[]> {
	if (!hasSession(event.cookies)) return Promise.resolve([]);

	return apiForEvent(event)
		.GET("/accounts/signed-in")
		.then(({ data }) => data ?? [])
		.catch(() => []);
}

export function actingSlot(event: RequestEvent): Promise<string | null> {
	if (!hasSession(event.cookies)) return Promise.resolve(null);

	return event.locals.signedIn.then(
		(accounts) => actingOf(accounts, event.url, event.params.workspace)?.slot ?? null
	);
}
