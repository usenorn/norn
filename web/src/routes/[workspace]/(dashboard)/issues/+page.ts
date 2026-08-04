import { apiFor } from "$lib/api";
import { issueLayouts, issueTabs, type IssueLayout, type IssueTab } from "$lib/issues/board";
import type { Issue, IssueProgress } from "$lib/issues/board";
import type { Team } from "$lib/team/teams";
import type { WorkflowState } from "$lib/team/states";
import type { components } from "$lib/api/dashboard.gen";
import type { PageLoad } from "./$types";

export type Member = components["schemas"]["Membership"];

export type IssuesPageData = {
	team: Team | null;
	teams: Team[];
	issues: Issue[] | undefined;
	states: WorkflowState[] | undefined;
	progress: IssueProgress | undefined;
	members: Member[];
	tab: IssueTab;
	layout: IssueLayout;
	showEmpty: boolean;
};

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export const load: PageLoad = async ({ fetch, url, parent }): Promise<IssuesPageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();
	const q = url.searchParams;

	const view = {
		tab: pick(q.get("tab"), issueTabs, "open"),
		layout: pick(q.get("layout"), issueLayouts, "list"),
		showEmpty: q.get("empty") === "1",
	};

	const available = teams ?? [];
	const requested = q.get("team")?.toUpperCase();
	const team =
		available.find((candidate) => candidate.key === requested) ?? available[0] ?? null;

	if (!team) {
		return {
			...view,
			team: null,
			teams: available,
			issues: undefined,
			states: undefined,
			progress: undefined,
			members: [],
		};
	}

	const path = { workspaceId: workspace.id };

	const [issues, states, progress, members] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/issues", {
			fetch,
			params: { path, query: { teamId: team.id, limit: 200 } },
		}),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			fetch,
			params: { path: { ...path, teamId: team.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/progress", {
			fetch,
			params: { path, query: { teamId: team.id } },
		}),
		api.GET("/workspaces/{workspaceId}/members", { fetch, params: { path } }),
	]);

	return {
		...view,
		team,
		teams: available,
		issues: issues.data?.issues,
		states: states.data,
		progress: progress.data,
		members: members.data?.members ?? [],
	};
};
