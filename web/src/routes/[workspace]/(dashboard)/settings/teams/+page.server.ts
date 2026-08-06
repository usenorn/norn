import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import { createTeamSchema } from "$lib/team/create-team-schema";
import {
	listingFor,
	teamKeyMessage,
	teamKeySuggestions,
	teamNameMessage,
	type Team,
	type TeamCreationFailure,
} from "$lib/team/teams";
import type { Actions, PageServerLoad } from "./$types";

type CreateTeamForm = Infer<typeof createTeamSchema>;
type CreateTeamOutcome = { kind: "created"; team: Team } | TeamCreationFailure;

export const load: PageServerLoad = async ({ depends, route, parent }) => {
	depends(keys.page(route.id));

	const { teams } = await parent();

	return {
		listing: listingFor(teams),
		form: await superValidate<CreateTeamForm, CreateTeamOutcome>(zod4(createTeamSchema)),
	};
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<CreateTeamForm, CreateTeamOutcome>(
			body,
			zod4(createTeamSchema)
		);

		if (!form.valid) return fail(400, { form });

		const path = { workspaceId: String(body.get("workspaceId") ?? "") };

		const created = await locals.api
			.POST("/workspaces/{workspaceId}/teams", {
				params: { path },
				body: {
					key: form.data.key,
					name: form.data.name,
					visibility: form.data.visibility,
				},
			})
			.catch(() => null);

		if (!created) return message(form, { kind: "unavailable" }, { status: 500 });
		if (created.data) return message(form, { kind: "created", team: created.data });

		const { error } = created;

		if (!error) return message(form, { kind: "unavailable" }, { status: 500 });
		if (error.status === 403) return message(form, { kind: "forbidden" }, { status: 403 });

		if (error.status === 409) {
			const existing = await locals.api.GET("/workspaces/{workspaceId}/teams", {
				params: { path },
			});

			return message(
				form,
				{
					kind: "key_taken",
					key: form.data.key,
					suggestions: teamKeySuggestions(
						form.data.name,
						form.data.key,
						(existing.data ?? []).map((team) => team.key)
					),
				},
				{ status: 409 }
			);
		}

		for (const field of error.errors ?? []) {
			if (field.field === "key") setError(form, "key", teamKeyMessage(field.code));
			if (field.field === "name") setError(form, "name", teamNameMessage(field.code));
		}

		return fail(400, { form });
	},
};
