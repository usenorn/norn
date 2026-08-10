import createClient from "openapi-fetch";
import { browser } from "$app/environment";
import { page } from "$app/state";
import { sessionHeader } from "$lib/account/accounts";
import type { paths } from "./dashboard.gen";

export const api = createClient<paths>({ baseUrl: "/v1" });

// A tab renders one page at a time, so the session that page loaded under is the one every call
// from it must act as. Reading it from page data rather than a module store keeps the browser and
// the server naming the same session by construction.
api.use({
	onRequest: ({ request }) => {
		if (!browser) return;

		const slot = page.data.acting?.slot;

		if (slot) request.headers.set(sessionHeader, slot);
	},
});
