import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";

import { keys } from "$lib/api/keys";
import { connectRepositoriesSchema } from "$lib/source-control/source-control-schema";
import {
	detailOf,
	sourceControlFailure,
	sourceControlPath,
	type AvailableSourceControlRepository,
	type SourceControlConnection,
} from "$lib/source-control/source-control";
import type { Actions, PageServerLoad } from "./$types";

export type ConnectRepositoriesView =
	| { kind: "no_connection" }
	| {
			kind: "choose";
			connections: SourceControlConnection[];
			chosen: SourceControlConnection;
			offered: AvailableSourceControlRepository[];
			/** True when the platform could not be asked, which is not the same as offering none. */
			offerUnreadable: boolean;
			connected: string[];
	  }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type ConnectRepositoriesPageData = {
	view: ConnectRepositoriesView;
	teams: { id: string; key: string; name: string }[];
};

export const load: PageServerLoad = async ({ depends, route, url, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.sourceControlConnect(workspace.id));

	const [listing, repositories, teams] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/repositories", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	const reachable = (teams.data ?? []).map((team) => ({
		id: team.id,
		key: team.key,
		name: team.name,
	}));

	const form = await superValidate(zod4(connectRepositoriesSchema));

	if (listing.error) {
		const kind = listing.response.status === 403 ? "forbidden" : "unavailable";

		return { view: { kind } as ConnectRepositoriesView, teams: reachable, form };
	}

	const connections = listing.data ?? [];

	if (connections.length === 0) {
		return { view: { kind: "no_connection" } as ConnectRepositoriesView, teams: reachable, form };
	}

	// One connection is preselected rather than left on an empty option, because leaving it empty
	// is what kept the offer from ever being read.
	const wanted = url.searchParams.get("connection");
	const chosen = connections.find((one) => one.id === wanted) ?? connections[0];

	form.data.connectionId = chosen.id;

	let offered: AvailableSourceControlRepository[] = [];
	let offerUnreadable = false;

	if (chosen.authKind === "app") {
		const available = await locals.api.GET(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}/available-repositories",
			{ params: { path: { workspaceId: workspace.id, connectionId: chosen.id } } },
		);

		offered = available.data ?? [];
		offerUnreadable = Boolean(available.error);
	}

	return {
		view: {
			kind: "choose",
			connections,
			chosen,
			offered,
			offerUnreadable,
			connected: (repositories.data ?? [])
				.filter((one) => one.connectionId === chosen.id)
				.map((one) => one.fullName),
		} as ConnectRepositoriesView,
		teams: reachable,
		form,
	};
};

export const actions: Actions = {
	default: async ({ request, locals, params }) => {
		// The action replaces the whole query string, so anything it needs travels in the body.
		const posted = await request.formData();
		const workspaceId = String(posted.get("workspaceId") ?? "");
		const form = await superValidate(posted, zod4(connectRepositoriesSchema));

		if (!form.valid) return fail(400, { form });

		const refused: string[] = [];

		for (const fullName of form.data.fullNames) {
			const { data: added, error } = await locals.api.POST(
				"/workspaces/{workspaceId}/source-control/repositories",
				{
					params: { path: { workspaceId } },
					body: {
						connectionId: form.data.connectionId,
						fullName,
						mirrorLabel: form.data.mirrorLabel || undefined,
					},
				},
			);

			if (error || !added) {
				const said = detailOf(error, sourceControlFailure(error));

				refused.push(`${fullName} — ${said}`);

				continue;
			}

			if (!form.data.teamId) continue;

			// Routing is a narrowing, so it is only written when somebody asked for one.
			const { error: unrouted } = await locals.api.POST(
				"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}/routes",
				{
					params: { path: { workspaceId, repositoryId: added.repository.id } },
					body: { teamId: form.data.teamId, pathPrefix: form.data.pathPrefix || "" },
				},
			);

			if (unrouted) {
				refused.push(
					`${fullName} is connected, but routing it failed — ${detailOf(unrouted, sourceControlFailure(unrouted))}`,
				);
			}
		}

		// Every repository is reported by name: one opaque failure for a batch tells nobody which
		// of them landed.
		if (refused.length > 0) {
			return message(form, { kind: "partial", refused }, { status: 400 });
		}

		redirect(303, sourceControlPath(params.workspace));
	},
};
