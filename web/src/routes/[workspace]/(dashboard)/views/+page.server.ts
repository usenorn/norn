import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { viewSchema } from "$lib/views/view-schema";
import {
	listingFor,
	readViewFailure,
	type SavedViewSharing,
	type ViewFailure,
} from "$lib/views/views";
import type { Actions, PageServerLoad } from "./$types";

type ViewForm = Infer<typeof viewSchema>;

const formId = "saved-view-form";

export const load: PageServerLoad = async ({ parent }) => {
	const { views, teams } = await parent();

	return {
		listing: listingFor(views),
		teams: teams ?? [],
		form: await superValidate<ViewForm, ViewFailure>(zod4(viewSchema), { id: formId }),
	};
};

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const body = await request.formData();
		const form = await superValidate<ViewForm, ViewFailure>(body, zod4(viewSchema), { id: formId });

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");
		const savedViewId = String(body.get("savedViewId") ?? "");

		if (!workspaceId || !savedViewId) {
			return message(form, { kind: "unavailable" }, { status: 400 });
		}

		const { data, error } = await locals.api.PATCH(
			"/workspaces/{workspaceId}/saved-views/{savedViewId}",
			{
				params: { path: { workspaceId, savedViewId } },
				body: {
					name: form.data.name,
					sharing: form.data.sharing as SavedViewSharing,
					...(form.data.sharing === "team" ? { teamId: form.data.teamId } : {}),
				},
			}
		);

		if (error || !data) return message(form, readViewFailure(error), { status: 400 });

		const closed = new URL(url);
		closed.searchParams.delete("edit");
		closed.searchParams.delete("remove");

		redirect(303, `${closed.pathname}${closed.search}`);
	},
};
