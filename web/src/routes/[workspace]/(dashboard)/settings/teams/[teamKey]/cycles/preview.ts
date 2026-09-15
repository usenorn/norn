import type { CadenceSetting } from "$lib/cycles/cycles";

export type TeamCyclesPreview = { cadence: CadenceSetting };

export const teamCyclesPreviewStates: Record<string, TeamCyclesPreview> = import.meta.env.DEV
	? {
			cycles_off: { cadence: { kind: "disabled" } },
			cycles_on: {
				cadence: {
					kind: "enabled",
					cadence: {
						teamId: "00000000-0000-4000-8000-000000000101",
						lengthWeeks: 2,
						startsOn: 1,
						upcoming: [
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
							{
								id: "00000000-0000-4000-8000-000000000515",
								workspaceId: "00000000-0000-4000-8000-000000000001",
								teamId: "00000000-0000-4000-8000-000000000101",
								teamKey: "MOB",
								number: 26,
								name: "Cycle 26",
								startsOn: "2026-08-24",
								endsOn: "2026-09-06",
								phase: "upcoming",
							},
							{
								id: "00000000-0000-4000-8000-000000000516",
								workspaceId: "00000000-0000-4000-8000-000000000001",
								teamId: "00000000-0000-4000-8000-000000000101",
								teamKey: "MOB",
								number: 27,
								name: "Cycle 27",
								startsOn: "2026-09-07",
								endsOn: "2026-09-20",
								phase: "upcoming",
							},
						],
					},
				},
			},
			cycles_unavailable: { cadence: { kind: "unavailable" } },
		}
	: {};
