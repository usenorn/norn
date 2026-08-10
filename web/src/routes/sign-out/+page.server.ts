import { redirect } from "@sveltejs/kit";
import { sessionParam } from "$lib/account/accounts";
import type { Actions, PageServerLoad } from "./$types";

export const load: PageServerLoad = () => {
	redirect(307, "/");
};

export const actions: Actions = {
	default: async ({ locals, url }) => {
		const all = url.searchParams.get("scope") === "all";
		const slot = url.searchParams.get(sessionParam);

		// Resolved before the call, because the logout response rewrites the cookie jar the
		// directory would otherwise be read through.
		const before = await locals.signedIn;

		if (all) {
			await locals.api.DELETE("/accounts/signed-in");
			redirect(303, "/sign-in");
		}

		const client = slot ? locals.apiAs(slot) : locals.api;

		await client.POST("/auth/logout");

		const remaining = before.filter((account) => account.defaultSlot !== slot);

		redirect(303, remaining.length > 0 ? "/" : "/sign-in");
	},
};
