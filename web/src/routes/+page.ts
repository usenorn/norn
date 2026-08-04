import { redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, url}) => {
	const api = apiFor(url);

	const { data, error } = await api.GET("/workspaces", { fetch });

	if (error || !data) redirect(307, "/sign-in");

	const [first] = data;

	redirect(307, first ? `/${first.slug}` : "/create-workspace");
};
