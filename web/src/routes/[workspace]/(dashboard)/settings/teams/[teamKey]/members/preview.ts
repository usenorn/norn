import type { MemberFailure, TeamRoster } from "$lib/team/members";

export type TeamMembersPreview = {
	roster: TeamRoster;
	failure?: MemberFailure;
};

export const teamMembersPreviewStates: Record<string, TeamMembersPreview> = import.meta.env.DEV
	? {
			ready: {
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
			roster_loading: { roster: { kind: "loading" } },
			roster_empty: { roster: { kind: "empty" } },
			roster_unavailable: { roster: { kind: "unavailable" } },
			member_rejected: {
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
		}
	: {};
