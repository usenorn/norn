import { fail, redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import { sessionParam, withSlot } from "$lib/account/accounts";
import { lastWorkspaceCookie, rememberedWorkspace } from "$lib/account/last-workspace";
import { workspacePath } from "$lib/workspace/navigation";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import {
	createWorkspaceSchema,
	slugMessage,
	slugSuggestions,
} from "$lib/workspace/create-workspace-schema";
import { teamKeyMessage, teamKeySuggestions, teamNameMessage } from "$lib/team/teams";
import type { WorkspaceContext, WorkspaceCreationFailure } from "$lib/workspace/types";
import type { Actions, PageServerLoad } from "./$types";

type CreateWorkspaceForm = Infer<typeof createWorkspaceSchema>;

export const load: PageServerLoad = async ({ cookies, depends, route, parent }) => {
	depends(keys.page(route.id));

	const { accounts, acting } = await parent();
	const owner = accounts.find((candidate) => candidate.account.id === acting?.accountId);
	const held = owner?.workspaces ?? [];
	const remembered = owner && cookies.get(lastWorkspaceCookie(owner.account.id));
	const reachable = held.filter((candidate) => candidate.reachable);
	const leaving = rememberedWorkspace(reachable, remembered) ?? reachable[0];

	return {
		owner: owner?.account ?? null,
		choices: accounts.length,
		workspace: {
			existingWorkspace: (rememberedWorkspace(held, remembered) ?? held[0])?.workspace.name ?? null,
			returnTo: leaving
				? {
						name: leaving.workspace.name,
						href: withSlot(workspacePath(leaving.workspace.slug, "/my-tasks"), leaving.slot),
					}
				: null,
		} satisfies WorkspaceContext,
		form: await superValidate<CreateWorkspaceForm, WorkspaceCreationFailure>(
			zod4(createWorkspaceSchema)
		),
	};
};

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const slot = url.searchParams.get(sessionParam);

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

		if (data) {
			redirect(303, withSlot(`/invite-teammates?workspace=${data.slug}`, slot));
		}

		if (!error) return message(form, { kind: "unavailable" }, { status: 500 });

		if (error.status === 401) return message(form, { kind: "signed_out" }, { status: 401 });

		if ("code" in error && error.code === "team_key_taken") {
			return message(form, {
				kind: "team_key_taken",
				key: form.data.teamKey,
				suggestions: teamKeySuggestions(form.data.teamName, form.data.teamKey, []),
			});
		}

		if (error.status === 409 && !("code" in error)) {
			return message(form, {
				kind: "slug_taken",
				slug: form.data.slug,
				suggestions: slugSuggestions(form.data.slug),
			});
		}

		let handled = false;

		for (const field of error.errors ?? []) {
			if (field.field === "slug") setError(form, "slug", slugMessage(field.code));
			else if (field.field === "name") setError(form, "name", "Enter a workspace name.");
			else if (field.field === "team.key") setError(form, "teamKey", teamKeyMessage(field.code));
			else if (field.field === "team.name") setError(form, "teamName", teamNameMessage(field.code));
			else continue;

			handled = true;
		}

		if (handled) return fail(400, { form });

		return message(form, { kind: "unavailable" }, { status: 500 });
	},
};
