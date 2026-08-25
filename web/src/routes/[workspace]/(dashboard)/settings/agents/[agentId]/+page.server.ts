import type { ActivityFeed } from "$lib/activity/activity";
import { keys } from "$lib/api/keys";
import type { AgentRecord } from "$lib/agents/agent-record";
import type { PageServerLoad } from "./$types";

export type AgentRecordData = {
	record: AgentRecord;
	activity: ActivityFeed;
};

export const load: PageServerLoad = async ({
	depends,
	locals,
	params,
	parent,
}): Promise<AgentRecordData> => {
	const { workspace } = await parent();
	depends(keys.agent(workspace.id, params.agentId));

	const [agent, activity] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/agents/{agentId}", {
			params: { path: { workspaceId: workspace.id, agentId: params.agentId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/agents/{agentId}/activity", {
			params: {
				path: { workspaceId: workspace.id, agentId: params.agentId },
				query: { limit: 50 },
			},
		}),
	]);

	if (agent.error || !agent.data) {
		const status = agent.response.status;
		const record: AgentRecord =
			status === 403
				? { kind: "forbidden" }
				: status === 404
					? { kind: "missing" }
					: status === 409
						? { kind: "authority_missing" }
						: { kind: "unavailable" };

		return { record, activity: { kind: "unavailable" } };
	}

	const record: AgentRecord = { kind: "ready", value: agent.data };

	if (activity.error || !activity.data) {
		return { record, activity: { kind: "unavailable" } };
	}

	if (activity.data.events.length === 0) {
		return { record, activity: { kind: "empty" } };
	}

	return {
		record,
		activity: {
			kind: "ready",
			events: activity.data.events,
			nextCursor: activity.data.nextCursor,
		},
	};
};
