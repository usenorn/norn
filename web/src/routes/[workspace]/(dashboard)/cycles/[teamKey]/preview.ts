import type { CycleListing } from "$lib/cycles/cycles";

export type TeamCyclesPreview = { listing: CycleListing };

export const teamCyclesPreviewStates: Record<string, TeamCyclesPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			not_found: { listing: { kind: "not_found" } },
			disabled: { listing: { kind: "disabled", teamKey: "DES" } },
			upcoming_only: {
				listing: {
					kind: "ready",
					teamKey: "MOB",
					teamName: "Mobile",
					cycles: [
						{
							id: "00000000-0000-4000-8000-000000000501",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 1,
							name: "Cycle 1",
							startsOn: "2026-08-10",
							endsOn: "2026-08-23",
							phase: "upcoming",
						},
						{
							id: "00000000-0000-4000-8000-000000000502",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 2,
							name: "Cycle 2",
							startsOn: "2026-08-24",
							endsOn: "2026-09-06",
							phase: "upcoming",
						},
					],
				},
			},
			ready: {
				listing: {
					kind: "ready",
					teamKey: "MOB",
					teamName: "Mobile",
					cycles: [
						{
							id: "00000000-0000-4000-8000-000000000511",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 22,
							name: "Cycle 22",
							startsOn: "2026-06-29",
							endsOn: "2026-07-12",
							phase: "closed",
							closedAt: "2026-07-13T09:12:00Z",
							rollover: "next",
						},
						{
							id: "00000000-0000-4000-8000-000000000512",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 23,
							name: "Cycle 23",
							startsOn: "2026-07-13",
							endsOn: "2026-07-26",
							phase: "ended",
						},
						{
							id: "00000000-0000-4000-8000-000000000513",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 24,
							name: "Cycle 24",
							startsOn: "2026-07-27",
							endsOn: "2026-08-09",
							phase: "current",
						},
						{
							id: "00000000-0000-4000-8000-000000000514",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							number: 25,
							name: "Cycle 25",
							startsOn: "2026-08-10",
							endsOn: "2026-08-23",
							phase: "upcoming",
						},
					],
				},
			},
		}
	: {};
