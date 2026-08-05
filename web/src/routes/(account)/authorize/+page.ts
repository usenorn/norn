import { apiFor } from "$lib/api";
import type { ConsentState } from "$lib/account/connections";
import type { PageLoad } from "./$types";

export type AuthorizePageData = {
	consent: ConsentState;
	requestId: string;
};

export const load: PageLoad = async ({ fetch, url }): Promise<AuthorizePageData> => {
	const requestId = url.searchParams.get("request") ?? "";

	if (!requestId) return { consent: { kind: "expired" }, requestId };

	const api = apiFor(url);

	const described = await api.GET("/mcp/authorizations/{requestId}", {
		fetch,
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
