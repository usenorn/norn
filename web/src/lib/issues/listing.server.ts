import type { Cookies } from "@sveltejs/kit";
import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import { backlogStates } from "$lib/issues/board";
import {
	carriesDisplay,
	displayCookie,
	readDisplay,
	readLayout,
	readTab,
	writeDisplay,
} from "$lib/issues/display";
import { facetFilters, readFacets } from "$lib/issues/facets";
import { appliedView, viewQuery, viewTeams } from "$lib/views/applied";
import { calendarDate } from "$lib/time";
import type { SavedView } from "$lib/views/views";
import type { Team } from "$lib/team/teams";
import type { WorkflowState } from "$lib/team/states";
import type { IssuesListingData } from "./listing";

export type ListingScope =
	| { kind: "workspace"; views: SavedView[] }
	| { kind: "team"; team: Team };

export type ListingRequest = {
	api: Client<paths>;
	workspaceId: string;
	timezone: string;
	now: string;
	memberId: string;
	teams: Team[];
	states: WorkflowState[];
	scope: ListingScope;
	url: URL;
	cookies: Cookies;
};

function byPositionThenName(a: WorkflowState, b: WorkflowState): number {
	return a.position - b.position || a.name.localeCompare(b.name);
}

export async function issuesListing(request: ListingRequest): Promise<IssuesListingData> {
	const { api, workspaceId, timezone, now, memberId, teams, scope, url, cookies } = request;

	const q = url.searchParams;
	const remembered = displayCookie(memberId, workspaceId);
	const chosen = carriesDisplay(q) ? q : new URLSearchParams(cookies.get(remembered) ?? "");

	const facets = readFacets(q);
	const display = readDisplay(chosen);
	const layout = readLayout(chosen);
	const tab = readTab(chosen);
	const today = calendarDate(now, timezone);

	if (carriesDisplay(q)) {
		cookies.set(remembered, writeDisplay(display, layout, tab).toString(), {
			path: "/",
			httpOnly: true,
			sameSite: "lax",
			maxAge: 60 * 60 * 24 * 365,
		});
	}

	const options = { tab, layout, facets, display, today };

	const applied =
		scope.kind === "workspace"
			? appliedView(q.get("view") ?? "", scope.views, teams)
			: ({ kind: "none" } as const);

	const team =
		scope.kind === "team" ? scope.team : applied.kind === "applied" ? applied.team : null;

	if (teams.length === 0) {
		return {
			...options,
			applied,
			query: viewQuery(applied, options.tab, null),
			team: null,
			teams,
			issues: undefined,
			nextCursor: undefined,
			groups: undefined,
			totals: undefined,
			states: [],
			progress: undefined,
		};
	}

	const named = viewTeams(applied, teams);
	const within =
		named.length > 0
			? new Set(named.map((each) => each.id))
			: team
				? new Set([team.id])
				: null;

	const states = (within
		? request.states.filter((state) => within.has(state.teamId))
		: request.states
	)
		.slice()
		.sort(byPositionThenName);

	const path = { workspaceId };
	const backlog = backlogStates(states);
	const filters = facetFilters(facets, today);

	const query = viewQuery(applied, options.tab, team?.id ?? null, "", {
		filters,
		sort: display.ordering,
		grouping: display.grouping,
		backlogStateIds: backlog.map((state) => state.id),
	});

	const across = viewQuery(applied, "all", team?.id ?? null, "", { filters, grouping: "state" });

	const [issues, totals, progress, detail] = await Promise.all([
		api.POST("/workspaces/{workspaceId}/issues/query", { params: { path }, body: query }),
		api.POST("/workspaces/{workspaceId}/issues/query", {
			params: { path },
			body: { ...across, perGroup: undefined, limit: 1 },
		}),
		facets.cycle
			? api.GET("/workspaces/{workspaceId}/issues/progress", {
					params: { path, query: { cycleId: facets.cycle } },
				})
			: Promise.resolve({ data: undefined }),
		applied.kind === "applied"
			? api.GET("/workspaces/{workspaceId}/saved-views/{savedViewId}", {
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
		teams,
		issues: issues.data?.issues,
		nextCursor: issues.data?.nextCursor,
		groups: issues.data?.groups,
		totals: totals.data?.groups,
		states,
		progress: progress.data,
	};
}
