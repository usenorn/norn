import type { ActivityFeed } from "$lib/activity/activity";
import type { WorkspaceAgent } from "$lib/agents/agents";
import type { PageServerLoad } from "./$types";

export type AgentRecordData = {
	agent: WorkspaceAgent | null;
	activity: ActivityFeed;
};

export const load: PageServerLoad = async ({
	locals,
	params,
	parent,
}): Promise<AgentRecordData> => {
	const { workspace } = await parent();

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
