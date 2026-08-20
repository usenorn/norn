import type { TeamOverview } from "$lib/team/teams";

export type TeamOverviewPreview = { overview: TeamOverview };

export const teamOverviewPreviewStates: Record<string, TeamOverviewPreview> = import.meta.env.DEV
	? {
			loading: { overview: { kind: "loading" } },
			unavailable: { overview: { kind: "unavailable" } },
			not_found: { overview: { kind: "not_found" } },
			ready: {
				overview: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
			},
			private: {
				overview: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000102",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "DSG",
						name: "Design",
						visibility: "private",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
				},
			},
		}
	: {};
