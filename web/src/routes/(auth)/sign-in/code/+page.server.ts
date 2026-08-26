import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { signInCodeSchema } from "$lib/auth/sign-in-code-schema";
import { signInCodeFailure } from "$lib/auth/sign-in-code";
import { safeReturn } from "$lib/auth/return-to";
import { withSlot } from "$lib/account/accounts";
import { leaveIfSignedIn } from "$lib/auth/signed-in";
import type { SignInCodeFailure } from "$lib/auth/types";
import type { Actions, PageServerLoad } from "./$types";

type CodeForm = Infer<typeof signInCodeSchema>;

export const load: PageServerLoad = async ({ locals, url }) => {
	await leaveIfSignedIn(locals, url);

	const challengeId = url.searchParams.get("challenge") ?? "";

	if (!challengeId) redirect(303, "/sign-in");

	const form = await superValidate<CodeForm, SignInCodeFailure>(zod4(signInCodeSchema));

	form.data.challengeId = challengeId;

	return { form };
};

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const form = await superValidate<CodeForm, SignInCodeFailure>(request, zod4(signInCodeSchema));

		if (!form.valid) return fail(400, { form });

		const { data, error } = await locals.api.POST("/auth/login/verify", {
			body: { challengeId: form.data.challengeId, code: form.data.code },
		});

		if (error || !data) {
			form.data.code = "";

			return message(form, error ? signInCodeFailure(error) : { kind: "unavailable" }, {
				status: 401,
			});
		}

		redirect(303, withSlot(safeReturn(url.searchParams.get("return")), data.slot));
	},
};
