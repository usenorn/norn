import type { MemberFailure, TeamRoster } from "$lib/team/members";
import type { CadenceSetting } from "$lib/cycles/cycles";
import type { TeamNotificationSetting } from "$lib/notifications/notifications";
import type { TriageSetting } from "$lib/triage/triage";
import type { StateList } from "$lib/team/states";
import type { TeamSettings } from "$lib/team/team-settings";

export type TeamDetailPreview = {
	settings: TeamSettings;
	roster: TeamRoster;
	states?: StateList;
	cadence?: CadenceSetting;
	triage?: TriageSetting;
	notifications?: TeamNotificationSetting;
	failure?: MemberFailure;
	busy?: boolean;
};

export const teamDetailPreviewStates: Record<string, TeamDetailPreview> = import.meta.env.DEV
	? {
			loading: { settings: { kind: "loading" }, roster: { kind: "loading" } },
			ready: {
				settings: {
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
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
							joinedAt: "2026-01-05T09:00:00Z",
						},
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000202",
							displayName: "Tobias Lang",
							email: "tobias@northwind.co",
							joinedAt: "2026-01-06T09:00:00Z",
						},
					],
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
						visibility: "private",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
				},
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000102",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
							joinedAt: "2026-02-12T09:00:00Z",
						},
					],
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
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
							joinedAt: "2026-01-05T09:00:00Z",
						},
					],
				},
			},
			archived: {
				settings: {
					kind: "archived",
					team: {
						id: "00000000-0000-4000-8000-000000000103",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "LEG",
						name: "Legacy Billing",
						visibility: "public",
						status: "archived",
						createdAt: "2025-06-02T09:00:00Z",
						archivedAt: "2026-03-18T09:00:00Z",
					},
				},
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000103",
							accountId: "00000000-0000-4000-8000-000000000202",
							displayName: "Tobias Lang",
							email: "tobias@northwind.co",
							joinedAt: "2025-06-03T09:00:00Z",
						},
					],
				},
			},
			read_only: {
				settings: {
					kind: "read_only",
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
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
							joinedAt: "2026-01-05T09:00:00Z",
						},
					],
				},
			},
			roster_empty: {
				settings: {
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
				roster: { kind: "empty" },
			},
			roster_unavailable: {
				settings: {
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
				roster: { kind: "unavailable" },
			},
			member_rejected: {
				settings: {
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
				roster: {
					kind: "ready",
					members: [
						{
							teamId: "00000000-0000-4000-8000-000000000101",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Nadia Freeman",
							email: "nadia@northwind.co",
							joinedAt: "2026-01-05T09:00:00Z",
						},
					],
				},
				failure: { kind: "already_member" },
			},
			states_loading: {
				settings: {
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
				roster: { kind: "empty" },
				states: { kind: "loading" },
			},
			states_unavailable: {
				settings: {
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
				roster: { kind: "empty" },
				states: { kind: "unavailable" },
			},
			states_renamed: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000102",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "DSG",
						name: "Design",
						visibility: "public",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
				},
				roster: { kind: "empty" },
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
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000104",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "OPS",
						name: "Operations",
						visibility: "public",
						status: "active",
						createdAt: "2026-04-02T09:00:00Z",
					},
				},
				roster: { kind: "empty" },
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
			cycles_off: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000001",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				roster: { kind: "empty" },
				cadence: { kind: "disabled" },
			},
			cycles_on: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000001",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				roster: { kind: "empty" },
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
			cycles_unavailable: {
				settings: {
					kind: "ready",
					team: {
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000001",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				roster: { kind: "empty" },
				cadence: { kind: "unavailable" },
			},
			not_found: { settings: { kind: "not_found" }, roster: { kind: "unavailable" } },
			unavailable: { settings: { kind: "unavailable" }, roster: { kind: "unavailable" } },
		}
	: {};
