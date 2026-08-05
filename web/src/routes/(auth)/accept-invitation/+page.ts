import { apiFor } from "$lib/api";
import {
	invitationState,
	linkFailure,
	type AcceptInvitation,
} from "$lib/workspace/accept-invitation";
import { acceptInvitationPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

type AcceptInvitationData = {
	token: string | null;
	invitation: AcceptInvitation;
	signedInAs: string;
};

export const load: PageLoad = async ({ fetch, url }): Promise<AcceptInvitationData> => {
	const api = apiFor(url);

	const token = url.searchParams.get("token");

	if (import.meta.env.DEV && acceptInvitationPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, invitation: { kind: "no_token" }, signedInAs: "" };
	}

	if (!token) return { token: null, invitation: { kind: "no_token" }, signedInAs: "" };

	const [preview, account] = await Promise.all([
		api.POST("/invitations/preview", { fetch, body: { token } }),
		api.GET("/accounts/me", { fetch }),
	]);

	const signedInAs = account.data?.displayName ?? account.data?.email ?? "";

	if (preview.error) return { token, invitation: linkFailure(preview.error), signedInAs };
	if (!preview.data) return { token, invitation: { kind: "unavailable" }, signedInAs };

	return {
		token,
		invitation: invitationState(preview.data, account.data?.email ?? null),
		signedInAs,
	};
};
