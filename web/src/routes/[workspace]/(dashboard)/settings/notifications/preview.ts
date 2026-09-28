import type { MemberTelegramBots, TelegramOutcome } from "$lib/agents/telegram";
import type { NotificationSettingsPanel } from "./+page.server";

export type NotificationSettingsPreview = {
	panel?: NotificationSettingsPanel;
	saved?: boolean;
	telegram?: MemberTelegramBots;
	telegramOutcome?: TelegramOutcome;
};

export const notificationSettingsPreviewStates: Record<string, NotificationSettingsPreview> = import.meta
	.env.DEV
	? {
			loading: { panel: { kind: "loading" } },
			unavailable: { panel: { kind: "unavailable" } },
			ready: {
				panel: {
					kind: "ready",
					settings: {
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
							agents: { inbox: true, email: true },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
							agents: { inbox: true, email: true },
						},
					},
				},
			},
			no_email: {
				panel: {
					kind: "ready",
					settings: {
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
							agents: { inbox: true, email: true },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: true, email: false },
							stateChanged: { inbox: true, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
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
						overridden: false,
						preferences: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: false, email: false },
							stateChanged: { inbox: false, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
							agents: { inbox: false, email: false },
						},
						workspace: {
							assigned: { inbox: true, email: true },
							mentioned: { inbox: true, email: true },
							commented: { inbox: false, email: false },
							stateChanged: { inbox: false, email: false },
							membership: { inbox: true, email: false },
							approvals: { inbox: true, email: true },
							agents: { inbox: false, email: false },
						},
					},
				},
			},
			telegram_loading: { telegram: { kind: "loading" } },
			telegram_none: { telegram: { kind: "none" } },
			telegram_forbidden: { telegram: { kind: "forbidden" } },
			telegram_unavailable: { telegram: { kind: "unavailable" } },
			telegram_listed: {
				telegram: {
					kind: "listed",
					bots: [
						{
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							username: "northwind_triage_bot",
							linked: true,
							linkedAs: "raechen",
						},
						{
							agentId: "00000000-0000-4000-8000-0000000009c2",
							agentName: "workspace-guide with a name long enough to wrap on a narrow phone",
							username: "workspace_guide_bot",
							linked: false,
						},
					],
				},
			},
			telegram_invited: {
				telegram: {
					kind: "listed",
					bots: [
						{
							agentId: "00000000-0000-4000-8000-0000000009c2",
							agentName: "workspace-guide",
							username: "workspace_guide_bot",
							linked: false,
						},
					],
				},
				telegramOutcome: {
					kind: "invited",
					purpose: "private",
					invite: {
						url: "https://t.me/workspace_guide_bot?start=preview-member-code",
						expiresAt: "2026-09-28T09:15:00Z",
					},
				},
			},
			telegram_unlinked: {
				telegram: { kind: "none" },
				telegramOutcome: { kind: "unlinked" },
			},
			telegram_failed: {
				telegram: { kind: "none" },
				telegramOutcome: { kind: "failed", failure: { kind: "unavailable" } },
			},
		}
	: {};
