import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import type { Team } from "$lib/team/teams";
import { invitePreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

const memberScanLimit = 200;

export type InviteTarget = { id: string; slug: string; name: string; defaultTeamId: string | null };

export type InviteData = { target: InviteTarget; teams: Team[]; members: string[] };

export const load: PageServerLoad = async ({ depends, route, locals, url }): Promise<InviteData> => {
	depends(keys.page(route.id));

	if (import.meta.env.DEV && invitePreviewStates[url.searchParams.get("state") ?? ""]) {
		return {
			target: {
				id: "00000000-0000-4000-8000-000000000000",
				slug: "northwind",
				name: "Northwind",
				defaultTeamId: "00000000-0000-4000-8000-000000000101",
			},
			teams: [
				{
					id: "00000000-0000-4000-8000-000000000101",
					workspaceId: "00000000-0000-4000-8000-000000000000",
					key: "MOB",
					name: "Mobile",
					visibility: "public",
					status: "active",
					createdAt: "2026-01-04T09:00:00Z",
				},
				{
					id: "00000000-0000-4000-8000-000000000102",
					workspaceId: "00000000-0000-4000-8000-000000000000",
					key: "PLT",
					name: "Data Platform",
					visibility: "private",
					status: "active",
					createdAt: "2026-02-11T09:00:00Z",
				},
			],
			members: ["jun@northwind.co"],
		};
	}

	const { data, error } = await locals.api.GET("/workspaces");

	if (error || !data) redirect(307, "/sign-in");

	const slug = url.searchParams.get("workspace");
	const target = (slug && data.find((workspace) => workspace.slug === slug)) || data[0];

	if (!target) redirect(307, "/create-workspace");

	const [teams, members] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: target.id }, query: { status: "active" } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/members", {
			params: { path: { workspaceId: target.id }, query: { limit: memberScanLimit } },
		}),
	]);

	return {
		target: {
			id: target.id,
			slug: target.slug,
			name: target.name,
			defaultTeamId: target.defaultTeamId ?? null,
		},
		teams: teams.data ?? [],
		members: (members.data?.members ?? [])
			.map((member) => member.email?.toLowerCase())
			.filter((email): email is string => Boolean(email)),
	};
};
