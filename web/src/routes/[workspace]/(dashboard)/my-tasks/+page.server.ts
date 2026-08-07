import { keys } from "$lib/api/keys";
import type { Issue } from "$lib/issues/issues";
import { issuePageSize } from "$lib/issues/filter";
import type { IssueQueryBody } from "$lib/issues/filter";
import type { PageServerLoad } from "./$types";

const openCategories = ["not_started", "active"];

export type MyTasksPageData = {
	query: IssueQueryBody;
	issues: Issue[];
	nextCursor: string | undefined;
	assignee: string;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<MyTasksPageData> => {
	depends(keys.page(route.id));

	const { workspace, member } = await parent();

	depends(keys.issues(workspace.id));

	const query: IssueQueryBody = {
		filter: {
			all: [
				{ field: "assignee", op: "is", values: [member.id] },
				{ field: "stateCategory", op: "in", values: openCategories },
			],
		},
		sort: [{ field: "dueOn" }, { field: "priority" }],
		limit: issuePageSize,
	};

	const assigned = await locals.api.POST("/workspaces/{workspaceId}/issues/query", {
		params: { path: { workspaceId: workspace.id } },
		body: query,
	});

	return {
		query,
		issues: assigned.data?.issues ?? [],
		nextCursor: assigned.data?.nextCursor,
		assignee: member.name,
	};
};
