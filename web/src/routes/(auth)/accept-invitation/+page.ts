import { apiFor } from "$lib/api";
import {
	invitationState,
	linkFailure,
	type AcceptInvitation,
} from "$lib/workspace/accept-invitation";
import { acceptInvitationPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

type AcceptInvitationData = { token: string | null; invitation: AcceptInvitation };

export const load: PageLoad = async ({ fetch, url }): Promise<AcceptInvitationData> => {
	const api = apiFor(url);

	const token = url.searchParams.get("token");

	if (import.meta.env.DEV && acceptInvitationPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, invitation: { kind: "no_token" } };
	}

	if (!token) return { token: null, invitation: { kind: "no_token" } };

	const [preview, account] = await Promise.all([
		api.POST("/invitations/preview", { fetch, body: { token } }),
		api.GET("/accounts/me", { fetch }),
	]);

	if (preview.error) return { token, invitation: linkFailure(preview.error) };
	if (!preview.data) return { token, invitation: { kind: "unavailable" } };

	return {
		token,
		invitation: invitationState(preview.data, account.data?.email ?? null),
	};
};
