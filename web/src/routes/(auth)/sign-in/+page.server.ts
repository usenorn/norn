import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { signInSchema } from "$lib/auth/sign-in-schema";
import { signInFailure } from "$lib/auth/sign-in";
import { safeReturn } from "$lib/auth/return-to";
import { leaveIfSignedIn } from "$lib/auth/signed-in";
import { reachWorkspaceSignIn, type WorkspaceEntry } from "$lib/auth/workspace-sign-in";
import type { SignInFailure } from "$lib/auth/types";
import type { Actions, PageServerLoad } from "./$types";

type SignInForm = Infer<typeof signInSchema>;

export const load: PageServerLoad = async ({ locals, url }) => {
	await leaveIfSignedIn(locals.api);

	return {
		entry: (await reachWorkspaceSignIn(
			locals.api,
			url.searchParams.get("workspace") ?? ""
		)) as WorkspaceEntry,
		form: await superValidate<SignInForm, SignInFailure>(zod4(signInSchema)),
	};
};

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const form = await superValidate<SignInForm, SignInFailure>(request, zod4(signInSchema));

		const password = form.data.password;

		form.data.password = "";

		if (!form.valid) return fail(400, { form });

		const { error } = await locals.api.POST("/auth/login", {
			body: { email: form.data.email, password },
		});

		if (error) {
			return message(form, signInFailure(error) ?? { kind: "unavailable" }, { status: 401 });
		}

		redirect(303, safeReturn(url.searchParams.get("return")));
	},
};
