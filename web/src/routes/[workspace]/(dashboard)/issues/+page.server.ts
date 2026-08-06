import {
	backlogStates,
	issueLayouts,
	issueTabs,
	type IssueLayout,
	type IssueTab,
} from "$lib/issues/board";
import { keys } from "$lib/api/keys";
import type { Issue, IssueProgress } from "$lib/issues/board";
import {
	carriesDisplay,
	displayCookie,
	readDisplay,
	writeDisplay,
	type Display,
} from "$lib/issues/display";
import { facetFilters, readFacets, type Facets } from "$lib/issues/facets";
import type { IssueGroupTally, IssueQueryBody } from "$lib/issues/filter";
import type { Label } from "$lib/labels/labels";
import { appliedView, viewQuery, type AppliedView } from "$lib/views/applied";
import { calendarDate } from "$lib/time";
import type { Team } from "$lib/team/teams";
import type { WorkflowState } from "$lib/team/states";
import type { components } from "$lib/api/dashboard.gen";
import type { PageServerLoad } from "./$types";

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

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	url,
	cookies,
	parent,
}): Promise<IssuesPageData> => {
	depends(keys.page(route.id));

	const { workspace, teams, views, now } = await parent();

	depends(keys.issues(workspace.id));

	const q = url.searchParams;
	const remembered = displayCookie(workspace.id);
	const chosen = carriesDisplay(q) ? q : new URLSearchParams(cookies.get(remembered) ?? "");

	const facets = readFacets(q);
	const display = readDisplay(chosen);
	const layout = pick(chosen.get("layout"), issueLayouts, "list");
	const today = calendarDate(now, workspace.timezone);

	if (carriesDisplay(q)) {
		cookies.set(remembered, writeDisplay(display, layout).toString(), {
			path: "/",
			httpOnly: true,
			sameSite: "lax",
			maxAge: 60 * 60 * 24 * 365,
		});
	}

	const options = {
		tab: pick(q.get("tab"), issueTabs, "active"),
		layout,
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
		? await locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
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
		locals.api.POST("/workspaces/{workspaceId}/issues/query", { params: { path }, body: query }),
		locals.api.POST("/workspaces/{workspaceId}/issues/query", {
			params: { path },
			body: { ...across, limit: 1 },
		}),
		facets.cycle
			? locals.api.GET("/workspaces/{workspaceId}/issues/progress", {
					params: { path, query: { cycleId: facets.cycle } },
				})
			: Promise.resolve({ data: undefined }),
		locals.api.GET("/workspaces/{workspaceId}/members", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/labels", { params: { path } }),
		applied.kind === "applied"
			? locals.api.GET("/workspaces/{workspaceId}/saved-views/{savedViewId}", {
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
