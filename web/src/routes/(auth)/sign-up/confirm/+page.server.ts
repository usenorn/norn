import { redirect } from "@sveltejs/kit";
import { signUpConfirmFailure } from "$lib/auth/sign-up";
import type { SignUpConfirmation } from "$lib/auth/types";
import { signUpConfirmPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type ConfirmSignUpData = { token: string | null; confirmation: SignUpConfirmation };

export const load: PageServerLoad = ({ url }): ConfirmSignUpData => {
	const token = url.searchParams.get("token");

	if (import.meta.env.DEV && signUpConfirmPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, confirmation: { kind: "no_token" } };
	}

	return { token, confirmation: token ? { kind: "confirming" } : { kind: "no_token" } };
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const token = String((await request.formData()).get("token") ?? "");

		if (!token) return { confirmation: { kind: "no_token" } as SignUpConfirmation };

		const { data, error } = await locals.api.POST("/auth/sign-up/confirm", { body: { token } });

		if (error) return { confirmation: signUpConfirmFailure(error) };
		if (!data) return { confirmation: { kind: "unavailable" } as SignUpConfirmation };

		redirect(303, "/");
	},
};
