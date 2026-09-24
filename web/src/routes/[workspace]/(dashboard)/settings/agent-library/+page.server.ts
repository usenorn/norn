import { keys } from "$lib/api/keys";
import { reachInstance } from "$lib/auth/instance";
import { listedLibrary, type AgentLibraryListing } from "$lib/agents/agent-capabilities";
import { connectMcpServer, mcpConnectForm, type McpConnectForm } from "$lib/agents/mcp-connect.server";
import type { Actions, PageServerLoad } from "./$types";

export type AgentLibraryData = {
	listing: AgentLibraryListing;
	agentNames: Record<string, string>;
	selfHosted: boolean;
	connectForm: McpConnectForm;
};

export const load: PageServerLoad = async ({ depends, locals, parent, url }): Promise<AgentLibraryData> => {
	const { workspace } = await parent();
	depends(keys.agentLibrary(workspace.id));
	depends(keys.agents(workspace.id));

	const path = { workspaceId: workspace.id };

	const [library, agents, instance, connectForm] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/agent-library", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/agents", { params: { path } }),
		reachInstance(locals.api, url),
		mcpConnectForm(workspace.id, url.pathname),
	]);

	const listing: AgentLibraryListing = library.data
		? listedLibrary(library.data)
		: library.response.status === 403
			? { kind: "forbidden" }
			: { kind: "unavailable" };

	const agentNames = Object.fromEntries((agents.data ?? []).map((owned) => [owned.agent.id, owned.agent.name]));

	return { listing, agentNames, selfHosted: instance.selfHosted, connectForm };
};

export const actions: Actions = {
	connect: connectMcpServer,
};
