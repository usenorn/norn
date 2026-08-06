import { keys } from "$lib/api/keys";
import { bucketsOf } from "$lib/tasks/tasks";
import type { TaskBucket } from "$lib/tasks/types";
import type { PageServerLoad } from "./$types";

const assignedLimit = 200;

const openCategories = ["not_started", "active"];

export type MyTasksPageData = { buckets: TaskBucket[] };

export const load: PageServerLoad = async ({
	depends,
	locals,
	parent,
}): Promise<MyTasksPageData> => {
	const { workspace, member, now } = await parent();

	depends(keys.issues(workspace.id));

	const assigned = await locals.api.POST("/workspaces/{workspaceId}/issues/query", {
		params: { path: { workspaceId: workspace.id } },
		body: {
			filter: {
				all: [
					{ field: "assignee", op: "is", values: [member.id] },
					{ field: "stateCategory", op: "in", values: openCategories },
				],
			},
			sort: [{ field: "dueOn" }, { field: "priority" }],
			limit: assignedLimit,
		},
	});

	return {
		buckets: bucketsOf(assigned.data?.issues ?? [], member.name, now, workspace.timezone),
	};
};
