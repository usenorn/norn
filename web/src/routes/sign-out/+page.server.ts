import { redirect } from "@sveltejs/kit";
import type { Actions, PageServerLoad } from "./$types";

export const load: PageServerLoad = () => {
	redirect(307, "/");
};

export const actions: Actions = {
	default: async ({ locals }) => {
		await locals.api.POST("/auth/logout");

		redirect(303, "/sign-in");
	},
};
