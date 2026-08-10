import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import { signUpConfirmFailure } from "$lib/auth/sign-up";
import { safeReturn } from "$lib/auth/return-to";
import { withSlot } from "$lib/account/accounts";
import type { SignUpConfirmation } from "$lib/auth/types";
import { signUpConfirmPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type ConfirmSignUpData = { token: string | null; confirmation: SignUpConfirmation };

export const load: PageServerLoad = ({ depends, route, url }): ConfirmSignUpData => {
	depends(keys.page(route.id));

	const token = url.searchParams.get("token");

	if (import.meta.env.DEV && signUpConfirmPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, confirmation: { kind: "no_token" } };
	}

	return { token, confirmation: token ? { kind: "confirming" } : { kind: "no_token" } };
};

export const actions: Actions = {
	default: async ({ locals, request, url }) => {
		const token = String((await request.formData()).get("token") ?? "");

		if (!token) return { confirmation: { kind: "no_token" } as SignUpConfirmation };

		const { data, error } = await locals.api.POST("/auth/sign-up/confirm", { body: { token } });

		if (error) return { confirmation: signUpConfirmFailure(error) };
		if (!data) return { confirmation: { kind: "unavailable" } as SignUpConfirmation };

		// The browser may already hold other sessions, and only the account this link just created
		// should be the one it lands in.
		redirect(303, withSlot(safeReturn(url.searchParams.get("return")), data.slot));
	},
};
