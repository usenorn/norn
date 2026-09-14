import type { TeamRoster } from "$lib/team/members";
import type { TeamOverview } from "$lib/team/teams";

export type TeamOverviewPreview = { overview: TeamOverview; roster?: TeamRoster };

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
			members: {
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Rae Okafor",
							email: "rae@northwind.co",
						},
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000202",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
						},
					],
				},
				overview: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						description: "Everything people touch on a phone.",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
			},
			nobody: {
				roster: { kind: "empty" },
				overview: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000103",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "OPS",
						name: "Operations",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "public",
						status: "active",
						createdAt: "2026-03-02T09:00:00Z",
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
		}
	: {};
