import { apiFor } from "$lib/api";
import {
	backlogStates,
	issueLayouts,
	issueTabs,
	type IssueLayout,
	type IssueTab,
} from "$lib/issues/board";
import type { Issue, IssueProgress } from "$lib/issues/board";
import { readDisplay, type Display } from "$lib/issues/display";
import { facetFilters, readFacets, type Facets } from "$lib/issues/facets";
import type { IssueGroupTally, IssueQueryBody } from "$lib/issues/filter";
import type { Label } from "$lib/labels/labels";
import { appliedView, viewQuery, type AppliedView } from "$lib/views/applied";
import { calendarDate } from "$lib/time";
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
	totals: IssueGroupTally[] | undefined;
	states: WorkflowState[] | undefined;
	labels: Label[];
	progress: IssueProgress | undefined;
	members: Member[];
	facets: Facets;
	display: Display;
	today: string;
	tab: IssueTab;
	layout: IssueLayout;
};

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export const load: PageLoad = async ({ fetch, url, parent }): Promise<IssuesPageData> => {
	const api = apiFor(url);

	const { workspace, teams, views, now } = await parent();
	const q = url.searchParams;

	const facets = readFacets(q);
	const display = readDisplay(q);
	const today = calendarDate(now, workspace.timezone);

	const options = {
		tab: pick(q.get("tab"), issueTabs, "active"),
		layout: pick(q.get("layout"), issueLayouts, "list"),
		facets,
		display,
		today,
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
			query: viewQuery(applied, options.tab, null),
			team: null,
			teams: available,
			issues: undefined,
			nextCursor: undefined,
			groups: undefined,
			totals: undefined,
			states: undefined,
			labels: [],
			progress: undefined,
			members: [],
		};
	}

	const path = { workspaceId: workspace.id };

	const states = team
		? await api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
				fetch,
				params: { path: { ...path, teamId: team.id } },
			})
		: { data: undefined };

	const backlog = backlogStates(states.data ?? []);
	const filters = facetFilters(facets, today);

	const query = viewQuery(applied, options.tab, team?.id ?? null, "", {
		filters,
		sort: display.ordering,
		grouping: display.grouping,
		backlogStateIds: backlog.map((state) => state.id),
	});

	const across = viewQuery(applied, "all", team?.id ?? null, "", { filters, grouping: "state" });

	const [issues, totals, progress, members, labels, detail] = await Promise.all([
		api.POST("/workspaces/{workspaceId}/issues/query", { fetch, params: { path }, body: query }),
		api.POST("/workspaces/{workspaceId}/issues/query", {
			fetch,
			params: { path },
			body: { ...across, limit: 1 },
		}),
		facets.cycle
			? api.GET("/workspaces/{workspaceId}/issues/progress", {
					fetch,
					params: { path, query: { cycleId: facets.cycle } },
				})
			: Promise.resolve({ data: undefined }),
		api.GET("/workspaces/{workspaceId}/members", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/labels", { fetch, params: { path } }),
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
		totals: totals.data?.groups,
		states: states.data,
		labels: labels.data ?? [],
		progress: progress.data,
		members: members.data?.members ?? [],
	};
};
