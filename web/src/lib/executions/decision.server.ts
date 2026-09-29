import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import type { DecisionRight } from "./reviews";

export async function readDecisionRight(
	api: Client<paths>,
	workspaceId: string,
	issueId: string
): Promise<DecisionRight> {
	const right = await api.GET("/workspaces/{workspaceId}/issues/{issueId}/decision-right", {
		params: { path: { workspaceId, issueId } },
	});

	return right.data ?? { canDecide: false };
}
