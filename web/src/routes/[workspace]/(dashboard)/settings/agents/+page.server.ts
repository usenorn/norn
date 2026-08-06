import type { AgentListing } from "$lib/agents/agents";
import type { components } from "$lib/api/dashboard.gen";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

export type Membership = components["schemas"]["Membership"];

export type AgentsPageData = {
	listing: AgentListing;
	people: Membership[];
	teams: Team[];
};

export const load: PageServerLoad = async ({ locals, parent }): Promise<AgentsPageData> => {
	const { workspace, teams } = await parent();

	const [agents, members] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/agents", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/members", {
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
