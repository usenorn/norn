import { apiFor } from "$lib/api";
import type { NotificationSettings } from "$lib/notifications/notifications";
import type { PageLoad } from "./$types";

export type NotificationSettingsPanel =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type NotificationSettingsPageData = { panel: NotificationSettingsPanel };

export const load: PageLoad = async ({
	fetch,
	parent,
	url,
}): Promise<NotificationSettingsPageData> => {
	const api = apiFor(url);

	const { workspace } = await parent();

	const settings = await api.GET("/workspaces/{workspaceId}/notification-settings", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (!settings.data) return { panel: { kind: "unavailable" } };

	return { panel: { kind: "ready", settings: settings.data } };
};
