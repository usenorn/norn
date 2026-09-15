import type { StateList } from "$lib/team/states";

export type TeamStatesPreview = { states: StateList };

export const teamStatesPreviewStates: Record<string, TeamStatesPreview> = import.meta.env.DEV
	? {
			states_loading: { states: { kind: "loading" } },
			states_unavailable: { states: { kind: "unavailable" } },
			states_renamed: {
				states: {
					kind: "ready",
					states: [
						{
							id: "00000000-0000-4000-8000-000000000301",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Icebox",
							category: "not_started",
							position: 1,
							isDefault: false,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000302",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Ready",
							category: "not_started",
							position: 2,
							isDefault: true,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000303",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Sketching",
							category: "active",
							position: 3,
							isDefault: false,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000304",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Critique",
							category: "active",
							position: 4,
							isDefault: false,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000305",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Shipped",
							category: "complete",
							position: 5,
							isDefault: false,
							isCompletion: true,
						},
						{
							id: "00000000-0000-4000-8000-000000000306",
							teamId: "00000000-0000-4000-8000-000000000102",
							name: "Dropped",
							category: "abandoned",
							position: 6,
							isDefault: false,
							isCompletion: false,
						},
					],
				},
			},
			states_pared_back: {
				states: {
					kind: "ready",
					states: [
						{
							id: "00000000-0000-4000-8000-000000000401",
							teamId: "00000000-0000-4000-8000-000000000104",
							name: "Queued",
							category: "not_started",
							position: 1,
							isDefault: true,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000402",
							teamId: "00000000-0000-4000-8000-000000000104",
							name: "Handling",
							category: "active",
							position: 2,
							isDefault: false,
							isCompletion: false,
						},
						{
							id: "00000000-0000-4000-8000-000000000403",
							teamId: "00000000-0000-4000-8000-000000000104",
							name: "Resolved",
							category: "complete",
							position: 3,
							isDefault: false,
							isCompletion: true,
						},
						{
							id: "00000000-0000-4000-8000-000000000404",
							teamId: "00000000-0000-4000-8000-000000000104",
							name: "Withdrawn",
							category: "abandoned",
							position: 4,
							isDefault: false,
							isCompletion: false,
						},
					],
				},
			},
		}
	: {};
