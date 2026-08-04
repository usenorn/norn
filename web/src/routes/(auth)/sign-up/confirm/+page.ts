import type { SignUpConfirmation } from "$lib/auth/types";
import { signUpConfirmPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

type ConfirmSignUpData = { token: string | null; confirmation: SignUpConfirmation };

export const load: PageLoad = ({ url }): ConfirmSignUpData => {
	const token = url.searchParams.get("token");

	if (import.meta.env.DEV && signUpConfirmPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, confirmation: { kind: "no_token" } };
	}

	return { token, confirmation: token ? { kind: "confirming" } : { kind: "no_token" } };
};
