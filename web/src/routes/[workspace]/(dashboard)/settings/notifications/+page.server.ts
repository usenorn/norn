import type { NotificationSettings } from "$lib/notifications/notifications";
import type { PageServerLoad } from "./$types";

export type NotificationSettingsPanel =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type NotificationSettingsPageData = { panel: NotificationSettingsPanel };

export const load: PageServerLoad = async ({
	locals,
	parent,
}): Promise<NotificationSettingsPageData> => {
	const { workspace } = await parent();

	const settings = await locals.api.GET("/workspaces/{workspaceId}/notification-settings", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (!settings.data) return { panel: { kind: "unavailable" } };

	return { panel: { kind: "ready", settings: settings.data } };
};
