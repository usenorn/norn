import type { NotificationSettings } from "$lib/notifications/notifications";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type NotificationSettingsPanel =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type NotificationSettingsPageData = { panel: NotificationSettingsPanel };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<NotificationSettingsPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const settings = await locals.api.GET("/workspaces/{workspaceId}/notification-settings", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (!settings.data) return { panel: { kind: "unavailable" } };

	return { panel: { kind: "ready", settings: settings.data } };
};
