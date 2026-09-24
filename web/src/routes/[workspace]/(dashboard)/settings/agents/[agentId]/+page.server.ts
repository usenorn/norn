import type { ActivityFeed } from "$lib/activity/activity";
import { keys } from "$lib/api/keys";
import { reachInstance } from "$lib/auth/instance";
import {
	listedCapabilities,
	listedLibrary,
	type AgentCapabilities,
	type AgentLibraryListing,
} from "$lib/agents/agent-capabilities";
import { connectMcpServer, mcpConnectForm, type McpConnectForm } from "$lib/agents/mcp-connect.server";
import type { AgentRecord } from "$lib/agents/agent-record";
import type { Actions, PageServerLoad } from "./$types";

export type AgentRecordData = {
	record: AgentRecord;
	activity: ActivityFeed;
	capabilities: AgentCapabilities;
	library: AgentLibraryListing;
	selfHosted: boolean;
	connectForm: McpConnectForm;
};

export const load: PageServerLoad = async ({
	depends,
	locals,
	params,
	parent,
	url,
}): Promise<AgentRecordData> => {
	const { workspace } = await parent();
	depends(keys.agent(workspace.id, params.agentId));
	depends(keys.agentCapabilities(workspace.id, params.agentId));
	depends(keys.agentLibrary(workspace.id));

	const path = { workspaceId: workspace.id, agentId: params.agentId };

	const [agent, activity, capabilities, library, instance, connectForm] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/agents/{agentId}", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/agents/{agentId}/activity", {
			params: { path, query: { limit: 50 } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/agents/{agentId}/capabilities", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/agent-library", {
			params: { path: { workspaceId: workspace.id } },
		}),
		reachInstance(locals.api, url),
		mcpConnectForm(workspace.id, `${url.pathname}?tab=capabilities`),
	]);

	const listedCapability: AgentCapabilities = capabilities.data
		? listedCapabilities(capabilities.data.skills, capabilities.data.mcpServers)
		: capabilities.response.status === 403 || capabilities.response.status === 404
			? { kind: "forbidden" }
			: { kind: "unavailable" };

	const listedLibraryItems: AgentLibraryListing = library.data
		? listedLibrary(library.data)
		: library.response.status === 403
			? { kind: "forbidden" }
			: { kind: "unavailable" };

	const shared = {
		capabilities: listedCapability,
		library: listedLibraryItems,
		selfHosted: instance.selfHosted,
		connectForm,
	};

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

		return { record, activity: { kind: "unavailable" }, ...shared };
	}

	const record: AgentRecord = { kind: "ready", value: agent.data };

	if (activity.error || !activity.data) {
		return { record, activity: { kind: "unavailable" }, ...shared };
	}

	if (activity.data.events.length === 0) {
		return { record, activity: { kind: "empty" }, ...shared };
	}

	return {
		record,
		activity: {
			kind: "ready",
			events: activity.data.events,
			nextCursor: activity.data.nextCursor,
		},
		...shared,
	};
};

export const actions: Actions = {
	connect: connectMcpServer,
};
