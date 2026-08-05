import { apiFor } from "$lib/api";
import type { ActivityFeed } from "$lib/activity/activity";
import type { WorkspaceAgent } from "$lib/agents/agents";
import type { PageLoad } from "./$types";

export type AgentRecordData = {
	agent: WorkspaceAgent | null;
	activity: ActivityFeed;
};

export const load: PageLoad = async ({ fetch, params, parent, url }): Promise<AgentRecordData> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const [agent, activity] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/agents/{agentId}", {
			fetch,
			params: { path: { workspaceId: workspace.id, agentId: params.agentId } },
		}),
		api.GET("/workspaces/{workspaceId}/agents/{agentId}/activity", {
			fetch,
			params: {
				path: { workspaceId: workspace.id, agentId: params.agentId },
				query: { limit: 50 },
			},
		}),
	]);

	if (activity.error || !activity.data) {
		return { agent: agent.data ?? null, activity: { kind: "unavailable" } };
	}

	if (activity.data.events.length === 0) {
		return { agent: agent.data ?? null, activity: { kind: "empty" } };
	}

	return {
		agent: agent.data ?? null,
		activity: {
			kind: "ready",
			events: activity.data.events,
			nextCursor: activity.data.nextCursor,
		},
	};
};
