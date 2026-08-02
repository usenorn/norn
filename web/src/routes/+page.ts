import { redirect } from "@sveltejs/kit";
import { api } from "$lib/api";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch }) => {
	const { data, error } = await api.GET("/workspaces", { fetch });

	if (error || !data) redirect(307, "/sign-in");

	const [first] = data;

	redirect(307, first ? `/${first.slug}` : "/create-workspace");
};
