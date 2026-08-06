import { fail } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import type { ConnectionFailure, ConnectionListing } from "$lib/account/connections";
import { narrowConnectionSchema } from "$lib/account/narrow-connection-schema";
import { keys } from "$lib/api/keys";
import type { Team } from "$lib/team/teams";
import type { Actions, PageServerLoad } from "./$types";

type NarrowForm = Infer<typeof narrowConnectionSchema>;

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspaces } = await parent();

	const [connections, ...rosters] = await Promise.all([
		locals.api.GET("/mcp/connections"),
		...workspaces.map((workspace) =>
			locals.api.GET("/workspaces/{workspaceId}/teams", {
				params: { path: { workspaceId: workspace.id } },
			})
		),
	]);

	const teams: Record<string, Team[]> = {};

	workspaces.forEach((workspace, index) => {
		teams[workspace.id] = rosters[index]?.data ?? [];
	});

	const form = await superValidate<NarrowForm, ConnectionFailure>(zod4(narrowConnectionSchema));

	if (connections.error) {
		return {
			teams,
			form,
			listing: {
				kind: connections.error.status === 403 ? "forbidden" : "unavailable",
			} as ConnectionListing,
		};
	}

	if (!connections.data || connections.data.length === 0) {
		return { teams, form, listing: { kind: "empty" } as ConnectionListing };
	}

	return {
		teams,
		form,
		listing: { kind: "ready", connections: connections.data } as ConnectionListing,
	};
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<NarrowForm, ConnectionFailure>(
			body,
			zod4(narrowConnectionSchema)
		);

		if (!form.valid) return fail(400, { form });

		const connectionId = String(body.get("connectionId") ?? "");

		if (!connectionId) return message(form, { kind: "unavailable" }, { status: 400 });

		const { data: narrowed, error } = await locals.api.PATCH("/mcp/connections/{connectionId}", {
			params: { path: { connectionId } },
			body: {
				capability: form.data.capability,
				grants: form.data.followsMembership
					? undefined
					: form.data.grants.map((grant) => ({
							workspaceId: grant.workspaceId,
							allTeams: grant.allTeams,
							teamIds: grant.allTeams ? undefined : grant.teamIds,
						})),
			},
		});

		if (narrowed && !error) return { form };
		if (error?.status === 422) return message(form, { kind: "grant_invalid" }, { status: 422 });
		if (error?.status === 403) return message(form, { kind: "forbidden" }, { status: 403 });

		return message(form, { kind: "unavailable" }, { status: 500 });
	},
};
