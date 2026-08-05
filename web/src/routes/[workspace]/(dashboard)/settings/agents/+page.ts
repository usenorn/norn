import { apiFor } from "$lib/api";
import type { AgentListing } from "$lib/agents/agents";
import type { components } from "$lib/api/dashboard.gen";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type Membership = components["schemas"]["Membership"];

export type AgentsPageData = {
	listing: AgentListing;
	people: Membership[];
	teams: Team[];
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<AgentsPageData> => {
	const api = apiFor(url);
	const { workspace, teams } = await parent();

	const [agents, members] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/agents", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/members", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { limit: 200 } },
		}),
	]);

	const people = (members.data?.members ?? []).filter((member) => member.kind !== "agent");
	const reachable = (teams ?? []).filter((team) => team.status === "active");

	if (agents.error) {
		return {
			people,
			teams: reachable,
			listing: { kind: agents.error.status === 403 ? "forbidden" : "unavailable" },
		};
	}

	if (!agents.data || agents.data.length === 0) {
		return { people, teams: reachable, listing: { kind: "empty" } };
	}

	return { people, teams: reachable, listing: { kind: "ready", agents: agents.data } };
};
