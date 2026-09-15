import type { TeamSettings } from "$lib/team/team-settings";

export type TeamGeneralPreview = {
	settings: TeamSettings;
	busy?: boolean;
};

export const teamGeneralPreviewStates: Record<string, TeamGeneralPreview> = import.meta.env.DEV
	? {
			dressed: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						description: "Ships the phone app and its release train.",
						icon: "🚀",
						iconColor: "cyan",
						estimation: "sizes",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
			},
			private: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000102",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "PLT",
						name: "Data Platform",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "private",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
				},
			},
			saved: {
				settings: {
					kind: "saved",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile Apps",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
			},
			busy: {
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
				busy: true,
			},
		}
	: {};
