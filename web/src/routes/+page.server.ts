import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ locals }) => {
	const { data, error } = await locals.api.GET("/workspaces");

	if (error || !data) redirect(307, "/sign-in");

	const [first] = data;

	redirect(307, first ? `/${first.slug}` : "/create-workspace");
};
