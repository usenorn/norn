import { apiFor } from "$lib/api";
import { issueLayouts, issueTabs, type IssueLayout, type IssueTab } from "$lib/issues/board";
import type { Issue, IssueProgress } from "$lib/issues/board";
import type { IssueGroupTally, IssueQueryBody } from "$lib/issues/filter";
import { appliedView, viewQuery, type AppliedView } from "$lib/views/applied";
import type { Team } from "$lib/team/teams";
import type { WorkflowState } from "$lib/team/states";
import type { components } from "$lib/api/dashboard.gen";
import type { PageLoad } from "./$types";

export type Member = components["schemas"]["Membership"];

export type IssuesPageData = {
	team: Team | null;
	teams: Team[];
	applied: AppliedView;
	query: IssueQueryBody;
	issues: Issue[] | undefined;
	nextCursor: string | undefined;
	groups: IssueGroupTally[] | undefined;
	states: WorkflowState[] | undefined;
	progress: IssueProgress | undefined;
	members: Member[];
	tab: IssueTab;
	layout: IssueLayout;
	showEmpty: boolean;
	text: string;
};

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export const load: PageLoad = async ({ fetch, url, parent }): Promise<IssuesPageData> => {
	const api = apiFor(url);

	const { workspace, teams, views } = await parent();
	const q = url.searchParams;

	const options = {
		tab: pick(q.get("tab"), issueTabs, "open"),
		layout: pick(q.get("layout"), issueLayouts, "list"),
		showEmpty: q.get("empty") === "1",
		text: q.get("q") ?? "",
	};

	const available = teams ?? [];
	const applied = appliedView(q.get("view") ?? "", views ?? [], available);
	const requested = q.get("team")?.toUpperCase();

	const team =
		applied.kind === "applied"
			? applied.team
			: (available.find((candidate) => candidate.key === requested) ?? available[0] ?? null);

	if (available.length === 0) {
		return {
			...options,
			applied,
			query: viewQuery(applied, options.tab, null, options.text),
			team: null,
			teams: available,
			issues: undefined,
			nextCursor: undefined,
			groups: undefined,
			states: undefined,
			progress: undefined,
			members: [],
		};
	}

	const query = viewQuery(applied, options.tab, team?.id ?? null, options.text);

	const path = { workspaceId: workspace.id };

	const [issues, states, progress, members, detail] = await Promise.all([
		api.POST("/workspaces/{workspaceId}/issues/query", { fetch, params: { path }, body: query }),
		team
			? api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
					fetch,
					params: { path: { ...path, teamId: team.id } },
				})
			: Promise.resolve({ data: undefined }),
		team
			? api.GET("/workspaces/{workspaceId}/issues/progress", {
					fetch,
					params: { path, query: { teamId: team.id } },
				})
			: Promise.resolve({ data: undefined }),
		api.GET("/workspaces/{workspaceId}/members", { fetch, params: { path } }),
		applied.kind === "applied"
			? api.GET("/workspaces/{workspaceId}/saved-views/{savedViewId}", {
					fetch,
					params: { path: { ...path, savedViewId: applied.view.id } },
				})
			: Promise.resolve({ data: undefined }),
	]);

	return {
		...options,
		applied:
			applied.kind === "applied" && detail.data
				? { ...applied, references: detail.data.references }
				: applied,
		query,
		team,
		teams: available,
		issues: issues.data?.issues,
		nextCursor: issues.data?.nextCursor,
		groups: issues.data?.groups,
		states: states.data,
		progress: progress.data,
		members: members.data?.members ?? [],
	};
};
