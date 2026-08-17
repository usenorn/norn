import { backlogStates } from "$lib/issues/board";
import { keys } from "$lib/api/keys";
import type { Issue, IssueProgress } from "$lib/issues/board";
import {
	carriesDisplay,
	displayCookie,
	readDisplay,
	readLayout,
	readTab,
	readTeam,
	writeDisplay,
	type Display,
	type IssueLayout,
	type IssueTab,
} from "$lib/issues/display";
import { facetFilters, readFacets, type Facets } from "$lib/issues/facets";
import type { IssueGroupTally, IssueQueryBody } from "$lib/issues/filter";
import type { Label } from "$lib/labels/labels";
import { appliedView, viewQuery, viewTeams, type AppliedView } from "$lib/views/applied";
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

function byPositionThenName(a: WorkflowState, b: WorkflowState): number {
	return a.position - b.position || a.name.localeCompare(b.name);
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

	const { workspace, teams, views, now, member } = await parent();

	depends(keys.issues(workspace.id));

	const q = url.searchParams;
	const remembered = displayCookie(member.id, workspace.id);
	const stored = new URLSearchParams(cookies.get(remembered) ?? "");
	const chosen = carriesDisplay(q) ? q : stored;

	const facets = readFacets(q);
	const display = readDisplay(chosen);
	const layout = readLayout(chosen);
	const tab = readTab(chosen);
	const today = calendarDate(now, workspace.timezone);

	const options = { tab, layout, facets, display, today };

	const available = teams ?? [];
	const applied = appliedView(q.get("view") ?? "", views ?? [], available);
	const requested = readTeam(q) ?? readTeam(stored);

	const team =
		applied.kind === "applied"
			? applied.team
			: (available.find((candidate) => candidate.key === requested) ?? available[0] ?? null);

	const keep = team && applied.kind !== "applied" ? team.key : readTeam(stored);
	const persist = writeDisplay(display, layout, tab, keep).toString();

	if (persist !== cookies.get(remembered)) {
		cookies.set(remembered, persist, {
			path: "/",
			httpOnly: true,
			sameSite: "lax",
			maxAge: 60 * 60 * 24 * 365,
		});
	}

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

	const scoped = viewTeams(applied, available);
	const covered = scoped.length > 0 ? scoped : team ? [team] : [];

	const fetched = await Promise.all(
		covered.map((each) =>
			locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
				params: { path: { ...path, teamId: each.id } },
			})
		)
	);

	const states = fetched.some((result) => result.data)
		? fetched.flatMap((result) => result.data ?? []).sort(byPositionThenName)
		: undefined;

	const backlog = backlogStates(states ?? []);
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
			body: { ...across, perGroup: undefined, limit: 1 },
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
		states,
		labels: labels.data ?? [],
		progress: progress.data,
		members: members.data?.members ?? [],
	};
};
