import type { Team } from "$lib/team/teams";
import type { TriageFailure, TriageListing } from "$lib/triage/triage";

export type TriagePreview = {
	listing: TriageListing;
	teams?: Team[];
	failure?: TriageFailure;
};

export const triagePreviewStates: Record<string, TriagePreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			empty: { listing: { kind: "empty" } },
			ready: {
				teams: [
					{
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
				],
				listing: {
					kind: "ready",
					groups: [
						{
							waiting: 2,
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
							issues: [
								{
									id: "00000000-0000-4000-8000-000000000a01",
									workspaceId: "00000000-0000-4000-8000-000000000000",
									teamId: "00000000-0000-4000-8000-000000000101",
									teamKey: "MOB",
									referenceKey: "MOB",
									status: "active" as const,
									version: 1,
									description: "The uploader times out on a slow connection.",
									priority: "high",
									stateEnteredAt: "2026-08-04T09:00:00Z",
									state: {
										id: "00000000-0000-4000-8000-000000000301",
										name: "Ready",
										category: "not_started",
										position: 2,
									},
									labels: [],
									number: 31,
									reference: "MOB-31",
									title: "Avatar upload fails over 3G",
									triageState: "waiting" as const,
									triageSource: "token" as const,
									createdAt: "2026-08-04T09:00:00Z",
								},
								{
									id: "00000000-0000-4000-8000-000000000a02",
									workspaceId: "00000000-0000-4000-8000-000000000000",
									teamId: "00000000-0000-4000-8000-000000000101",
									teamKey: "MOB",
									referenceKey: "MOB",
									status: "active" as const,
									version: 1,
									description: "",
									priority: "none",
									stateEnteredAt: "2026-08-04T08:00:00Z",
									state: {
										id: "00000000-0000-4000-8000-000000000301",
										name: "Ready",
										category: "not_started",
										position: 2,
									},
									labels: [],
									number: 32,
									reference: "MOB-32",
									title: "Crash on the settings screen",
									triageState: "waiting" as const,
									triageSource: "agent" as const,
									createdAt: "2026-08-04T08:00:00Z",
								},
							],
						},
					],
				},
			},
			not_waiting: {
				failure: { kind: "not_waiting" },
				listing: {
					kind: "ready",
					groups: [
						{
							waiting: 1,
							team: null,
							issues: [
								{
									id: "00000000-0000-4000-8000-000000000a03",
									workspaceId: "00000000-0000-4000-8000-000000000000",
									teamId: "00000000-0000-4000-8000-000000000102",
									teamKey: "DSG",
									referenceKey: "DSG",
									status: "active" as const,
									version: 1,
									description: "",
									priority: "none",
									stateEnteredAt: "2026-08-03T09:00:00Z",
									state: {
										id: "00000000-0000-4000-8000-000000000401",
										name: "Ready",
										category: "not_started",
										position: 2,
									},
									labels: [],
									number: 8,
									reference: "DSG-8",
									title: "Someone else already decided this one",
									triageState: "waiting" as const,
									triageSource: "user" as const,
									createdAt: "2026-08-03T09:00:00Z",
								},
							],
						},
					],
				},
			},
		}
	: {};
