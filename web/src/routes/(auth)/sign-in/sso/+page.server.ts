import { fail, redirect } from "@sveltejs/kit";
import { setError, superValidate } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { safeReturn } from "$lib/auth/return-to";
import { workspaceEntrySchema } from "$lib/auth/workspace-entry-schema";
import { reachWorkspaceSignIn, ssoEntryPoint, workspaceSlug } from "$lib/auth/workspace-sign-in";
import type { Actions, PageServerLoad } from "./$types";

const unknownWorkspace = "No workspace at that address. Check it with whoever invited you.";

export const load: PageServerLoad = async () => ({
	form: await superValidate(zod4(workspaceEntrySchema)),
});

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const form = await superValidate(request, zod4(workspaceEntrySchema));

		if (!form.valid) return fail(400, { form });

		const slug = workspaceSlug(form.data.workspace);

		if (!slug) {
			return setError(form, "workspace", "Enter the workspace address you sign in at.");
		}

		const entry = await reachWorkspaceSignIn(locals.api, slug);

		if (entry.kind !== "ready") return setError(form, "workspace", unknownWorkspace);

		redirect(
			303,
			entry.signIn.sso
				? ssoEntryPoint(entry.signIn.workspace, safeReturn(url.searchParams.get("return")))
				: `/sign-in?workspace=${entry.signIn.workspace}`
		);
	},
};
