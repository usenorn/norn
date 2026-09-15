import { keys } from "$lib/api/keys";
import type { TeamNotificationSetting } from "$lib/notifications/notifications";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);
	const unavailable: TeamNotificationSetting = { kind: "unavailable" };

	if (!team) return { notifications: unavailable };

	const { data } = await locals.api.GET(
		"/workspaces/{workspaceId}/teams/{teamId}/notification-settings",
		{ params: { path: { workspaceId: workspace.id, teamId: team.id } } }
	);

	const notifications: TeamNotificationSetting = data
		? { kind: "ready", settings: data }
		: unavailable;

	return { notifications };
};
