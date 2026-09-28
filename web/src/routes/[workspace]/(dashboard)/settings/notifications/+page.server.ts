import type { NotificationSettings } from "$lib/notifications/notifications";
import type { MemberTelegramBots } from "$lib/agents/telegram";
import { keys } from "$lib/api/keys";
import { reachInstance } from "$lib/auth/instance";
import type { PageServerLoad } from "./$types";

export type NotificationSettingsPanel =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type NotificationSettingsPageData = {
	panel: NotificationSettingsPanel;
	telegram: MemberTelegramBots;
	selfHosted: boolean;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
	url,
}): Promise<NotificationSettingsPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();
	depends(keys.telegramBots(workspace.id));

	const params = { path: { workspaceId: workspace.id } };

	const [settings, bots, instance] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/notification-settings", { params }),
		locals.api.GET("/workspaces/{workspaceId}/telegram-bots", { params }),
		reachInstance(locals.api, url),
	]);

	const telegram: MemberTelegramBots = bots.data
		? bots.data.bots.length === 0
			? { kind: "none" }
			: { kind: "listed", bots: bots.data.bots }
		: bots.response.status === 403
			? { kind: "forbidden" }
			: { kind: "unavailable" };

	const shared = { telegram, selfHosted: instance.selfHosted };

	if (!settings.data) return { panel: { kind: "unavailable" }, ...shared };

	return { panel: { kind: "ready", settings: settings.data }, ...shared };
};
