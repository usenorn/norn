import type { NotificationSettingsPanel } from "./+page.server";

export type NotificationSettingsPreview = { panel: NotificationSettingsPanel; saved?: boolean };

export const notificationSettingsPreviewStates: Record<string, NotificationSettingsPreview> = import.meta
	.env.DEV
	? {
			loading: { panel: { kind: "loading" } },
			unavailable: { panel: { kind: "unavailable" } },
			ready: {
				panel: {
					kind: "ready",
					settings: {
						emailEnabled: true,
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: true, email: true },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: true, email: true },
						},
					},
				},
			},
			no_email: {
				panel: {
					kind: "ready",
					settings: {
						emailEnabled: false,
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: true, email: true },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: true, email: true },
						},
					},
				},
			},
			saved: {
				saved: true,
				panel: {
					kind: "ready",
					settings: {
						emailEnabled: true,
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: false, email: false },
							stateChanged: { inbox: false, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: false, email: false },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: false, email: false },
							stateChanged: { inbox: false, email: false },
							membership: { inbox: true, email: false },
							checks: { inbox: true, email: false },
							agents: { inbox: false, email: false },
						},
					},
				},
			},
		}
	: {};
