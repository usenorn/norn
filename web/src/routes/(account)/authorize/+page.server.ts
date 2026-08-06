import type { ConsentState } from "$lib/account/connections";
import type { PageServerLoad } from "./$types";

export type AuthorizePageData = {
	consent: ConsentState;
	requestId: string;
};

export const load: PageServerLoad = async ({ locals, url }): Promise<AuthorizePageData> => {
	const requestId = url.searchParams.get("request") ?? "";

	if (!requestId) return { consent: { kind: "expired" }, requestId };

	const described = await locals.api.GET("/mcp/authorizations/{requestId}", {
		params: { path: { requestId } },
	});

	if (described.error || !described.data) {
		return {
			consent: described.error?.status === 404 ? { kind: "expired" } : { kind: "failed" },
			requestId,
		};
	}

	return { consent: { kind: "ready", request: described.data }, requestId };
};
