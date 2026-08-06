import { fail, redirect } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import {
	createWorkspaceSchema,
	slugMessage,
	slugSuggestions,
} from "$lib/workspace/create-workspace-schema";
import { teamKeyMessage, teamKeySuggestions } from "$lib/team/teams";
import type { WorkspaceContext, WorkspaceCreationFailure } from "$lib/workspace/types";
import type { Actions, PageServerLoad } from "./$types";

type CreateWorkspaceForm = Infer<typeof createWorkspaceSchema>;

export const load: PageServerLoad = async ({ locals }) => {
	const { data } = await locals.api.GET("/workspaces");

	return {
		workspace: { existingWorkspace: data?.[0]?.name ?? null } as WorkspaceContext,
		form: await superValidate<CreateWorkspaceForm, WorkspaceCreationFailure>(
			zod4(createWorkspaceSchema)
		),
	};
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const form = await superValidate<CreateWorkspaceForm, WorkspaceCreationFailure>(
			request,
			zod4(createWorkspaceSchema)
		);

		if (!form.valid) return fail(400, { form });

		const { data, error } = await locals.api.POST("/workspaces", {
			body: {
				slug: form.data.slug,
				name: form.data.name,
				team: { key: form.data.teamKey, name: form.data.teamName },
			},
		});

		if (data) redirect(303, `/invite-teammates?workspace=${data.slug}`);

		if (error && "code" in error && error.code === "team_key_taken") {
			return message(form, {
				kind: "team_key_taken",
				key: form.data.teamKey,
				suggestions: teamKeySuggestions(form.data.teamName, form.data.teamKey, []),
			});
		}

		if (error?.status === 409) {
			return message(form, {
				kind: "slug_taken",
				slug: form.data.slug,
				suggestions: slugSuggestions(form.data.slug),
			});
		}

		for (const field of error?.errors ?? []) {
			if (field.field === "slug") setError(form, "slug", slugMessage(field.code));
			if (field.field === "name") setError(form, "name", "Enter a workspace name.");
			if (field.field === "key") setError(form, "teamKey", teamKeyMessage(field.code));
		}

		return fail(400, { form });
	},
};
