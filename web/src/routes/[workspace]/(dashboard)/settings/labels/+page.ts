import { apiFor } from "$lib/api";
import { boardFor, type LabelBoard } from "$lib/labels/labels";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type LabelsPageData = {
	board: LabelBoard;
	teams: Team[];
};

export const load: PageLoad = async ({ fetch, parent, url}): Promise<LabelsPageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();
	const path = { workspaceId: workspace.id };

	const [labels, groups] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/labels", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/label-groups", { fetch, params: { path } }),
	]);

	return {
		board: boardFor(labels.data, groups.data),
		teams: teams ?? [],
	};
};
