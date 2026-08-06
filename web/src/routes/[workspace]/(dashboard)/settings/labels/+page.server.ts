import { boardFor, type LabelBoard } from "$lib/labels/labels";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

export type LabelsPageData = {
	board: LabelBoard;
	teams: Team[];
};

export const load: PageServerLoad = async ({ locals, parent }): Promise<LabelsPageData> => {
	const { workspace, teams } = await parent();
	const path = { workspaceId: workspace.id };

	const [labels, groups] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/labels", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/label-groups", { params: { path } }),
	]);

	return {
		board: boardFor(labels.data, groups.data),
		teams: teams ?? [],
	};
};
