import type { TeamSettings } from "$lib/team/team-settings";

export type TeamSettingsPreview = {
	settings: TeamSettings;
	readOnly?: boolean;
};

export const teamSettingsPreviewStates: Record<string, TeamSettingsPreview> = import.meta.env.DEV
	? {
			loading: { settings: { kind: "loading" } },
			not_found: { settings: { kind: "not_found" } },
			unavailable: { settings: { kind: "unavailable" } },
			archived: {
				settings: {
					kind: "archived",
					team: {
						id: "00000000-0000-4000-8000-000000000103",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "LEG",
						name: "Legacy Billing",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "public",
						status: "archived",
						createdAt: "2025-06-02T09:00:00Z",
						archivedAt: "2026-03-18T09:00:00Z",
					},
				},
			},
			read_only: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				readOnly: true,
			},
		}
	: {};
