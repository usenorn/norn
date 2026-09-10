import { keys } from "$lib/api/keys";
import { listed, type Listed } from "$lib/api/listed";
import {
	carriesDisplay,
	displayCookie,
	readDisplay,
	sortFor,
	surfaceDefaults,
	writeDisplay,
	type Display,
} from "$lib/issues/display";
import { facetCount, facetFilters, readFacets, type Facets } from "$lib/issues/facets";
import { allOf, issuePageSize, type IssueFilter, type IssueQueryBody } from "$lib/issues/filter";
import type { Issue } from "$lib/issues/issues";
import { calendarDate } from "$lib/time";
import type { PageServerLoad } from "./$types";

const openCategories = ["not_started", "active"];

export type MyTasksPageData = {
	query: IssueQueryBody;
	rows: Listed<Issue>;
	facets: Facets;
	display: Display;
	today: string;
	assignee: string;
	assigneeId: string;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	url,
	cookies,
	locals,
	parent,
}): Promise<MyTasksPageData> => {
	depends(keys.page(route.id));

	const { workspace, member, now } = await parent();

	depends(keys.issues(workspace.id));

	const q = url.searchParams;
	const remembered = displayCookie("tasks", member.id, workspace.id);
	const chosen = carriesDisplay(q) ? q : new URLSearchParams(cookies.get(remembered) ?? "");

	const facets = readFacets(q);
	const display = readDisplay(chosen, surfaceDefaults.tasks);
	const today = calendarDate(now, workspace.timezone);

	if (carriesDisplay(q)) {
		cookies.set(remembered, writeDisplay(display).toString(), {
			path: "/",
			httpOnly: true,
			sameSite: "lax",
			maxAge: 60 * 60 * 24 * 365,
		});
	}

	const mine: IssueFilter[] = [
		{ field: "assignee", op: "is", values: [member.id] },
		{ field: "stateCategory", op: "in", values: openCategories },
	];

	const query: IssueQueryBody = {
		filter: allOf(...mine, ...facetFilters(facets, today)),
		sort: sortFor(display.ordering),
		limit: issuePageSize,
	};

	const assigned = await locals.api.POST("/workspaces/{workspaceId}/issues/query", {
		params: { path: { workspaceId: workspace.id } },
		body: query,
	});

	return {
		query,
		rows: listed(
			assigned.data && { data: { rows: assigned.data.issues, nextCursor: assigned.data.nextCursor } },
			facetCount(facets) > 0
		),
		facets,
		display,
		today,
		assignee: member.name,
		assigneeId: member.id,
	};
};
