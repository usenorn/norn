import { fail } from "@sveltejs/kit";
import { message, superValidate, type Infer, type SuperValidated } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import type { NotificationSettings } from "$lib/notifications/notifications";
import type { DecisionChannelOutcome } from "$lib/notifications/decision-channel";
import { decisionChannelSchema } from "$lib/notifications/decision-channel-schema";
import type { MemberTelegramBots } from "$lib/agents/telegram";
import { keys } from "$lib/api/keys";
import { reachInstance } from "$lib/auth/instance";
import type { Actions, PageServerLoad } from "./$types";

type DecisionChannelForm = Infer<typeof decisionChannelSchema>;

const decisionFormId = "decision-channel-form";

export type NotificationSettingsPanel =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type NotificationSettingsPageData = {
	panel: NotificationSettingsPanel;
	telegram: MemberTelegramBots;
	selfHosted: boolean;
	decisionForm: SuperValidated<DecisionChannelForm, DecisionChannelOutcome>;
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

	const [settings, bots, instance, channel] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/notification-settings", { params }),
		locals.api.GET("/workspaces/{workspaceId}/telegram-bots", { params }),
		reachInstance(locals.api, url),
		locals.api.GET("/workspaces/{workspaceId}/decision-channel", { params }),
	]);

	const telegram: MemberTelegramBots = bots.data
		? bots.data.bots.length === 0
			? { kind: "none" }
			: { kind: "listed", bots: bots.data.bots }
		: bots.response.status === 403
			? { kind: "forbidden" }
			: { kind: "unavailable" };

	const decisionForm = await superValidate<DecisionChannelForm, DecisionChannelOutcome>(
		{ channel: channel.data?.channel ?? "norn" },
		zod4(decisionChannelSchema),
		{ id: decisionFormId, errors: false }
	);

	const shared = { telegram, selfHosted: instance.selfHosted, decisionForm };

	if (!settings.data) return { panel: { kind: "unavailable" }, ...shared };

	return { panel: { kind: "ready", settings: settings.data }, ...shared };
};

export const actions: Actions = {
	decisionChannel: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<DecisionChannelForm, DecisionChannelOutcome>(
			body,
			zod4(decisionChannelSchema),
			{ id: decisionFormId }
		);

		if (!form.valid) return fail(400, { form });

		const { data } = await locals.api.PUT("/workspaces/{workspaceId}/decision-channel", {
			params: { path: { workspaceId: String(body.get("workspaceId") ?? "") } },
			body: { channel: form.data.channel },
		});

		if (!data) return message(form, { kind: "failed" }, { status: 400 });

		form.data.channel = data.channel;

		return message(form, { kind: "saved" });
	},
};
