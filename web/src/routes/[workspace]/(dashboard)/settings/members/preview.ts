import type {
	MemberAction,
	MemberListing,
	MemberPaging,
	MemberRemoval,
	MembershipFailure,
	MembershipNotice,
} from "$lib/workspace/members";

export type MembersPreview = {
	listing: MemberListing;
	paging?: MemberPaging;
	removal?: MemberRemoval;
	action?: MemberAction;
	failure?: MembershipFailure;
	notice?: MembershipNotice;
	viewerId?: string;
};

export const membersPreviewStates: Record<string, MembersPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" } },
			results: {
				viewerId: "00000000-0000-4000-8000-000000000301",
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000303",
							role: "viewer",
							source: "directory",
							displayName: "Milo Fernandes",
							email: "milo@meridian.co",
							joinedAt: "2026-03-02T09:00:00Z",
							lastActiveAt: "2026-08-01T09:00:00Z",
							lastAuthMethod: "sso",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000304",
							role: "member",
							source: "manual",
							displayName: "Sana Ali",
							email: "sana@meridian.co",
							joinedAt: "2026-04-18T09:00:00Z",
						},
					],
					nextCursor: "cursor-page-two",
				},
			},
			results_end: {
				viewerId: "00000000-0000-4000-8000-000000000301",
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			results_wide: {
				viewerId: "00000000-0000-4000-8000-000000000301",
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000305",
							role: "member",
							source: "manual",
							displayName: "Alexandrina Konstantinopoulou-Whitfield",
							email: "alexandrina.konstantinopoulou@meridian-industrial-group.example",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "sso",
						},
					],
				},
			},
			no_matches: { listing: { kind: "no_matches", query: "zzz" } },
			loading_more: {
				paging: { kind: "loading" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
					nextCursor: "cursor-page-two",
				},
			},
			more_unavailable: {
				paging: { kind: "unavailable" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
					nextCursor: "cursor-page-two",
				},
			},
			unavailable: { listing: { kind: "unavailable" } },
			role_changing: {
				action: { kind: "changing_role", accountId: "00000000-0000-4000-8000-000000000302" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			role_changed: {
				notice: { kind: "role_changed", name: "Jun Watanabe", role: "admin" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "admin",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			last_admin_demote: {
				failure: { kind: "last_admin", name: "Rae Okafor", intent: "demote" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			last_admin_remove: {
				failure: { kind: "last_admin", name: "Rae Okafor", intent: "remove" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			self_role: {
				failure: { kind: "self_role" },
				viewerId: "00000000-0000-4000-8000-000000000301",
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			directory_managed: {
				failure: { kind: "directory_managed", name: "Milo Fernandes" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000303",
							role: "viewer",
							source: "directory",
							displayName: "Milo Fernandes",
							email: "milo@meridian.co",
							joinedAt: "2026-03-02T09:00:00Z",
							lastActiveAt: "2026-08-01T09:00:00Z",
							lastAuthMethod: "sso",
						},
					],
				},
			},
			forbidden: {
				failure: { kind: "forbidden" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removal_loading: {
				removal: { kind: "loading", accountId: "00000000-0000-4000-8000-000000000302" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removal_confirm: {
				removal: {
					kind: "ready",
					soleAdmin: false,
					teams: [],
					member: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						accountId: "00000000-0000-4000-8000-000000000302",
						role: "member",
						source: "manual",
						displayName: "Jun Watanabe",
						email: "jun@meridian.co",
						joinedAt: "2026-02-11T09:00:00Z",
						lastActiveAt: "2026-07-20T09:00:00Z",
						lastAuthMethod: "password",
					},
				},
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000304",
							role: "member",
							source: "manual",
							displayName: "Sana Ali",
							email: "sana@meridian.co",
							joinedAt: "2026-04-18T09:00:00Z",
						},
					],
				},
			},
			removal_confirm_teams: {
				removal: {
					kind: "ready",
					soleAdmin: false,
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
						{
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
					],
					member: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						accountId: "00000000-0000-4000-8000-000000000302",
						role: "member",
						source: "manual",
						displayName: "Jun Watanabe",
						email: "jun@meridian.co",
						joinedAt: "2026-02-11T09:00:00Z",
						lastActiveAt: "2026-07-20T09:00:00Z",
						lastAuthMethod: "password",
					},
				},
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000304",
							role: "member",
							source: "manual",
							displayName: "Sana Ali",
							email: "sana@meridian.co",
							joinedAt: "2026-04-18T09:00:00Z",
						},
					],
				},
			},
			removal_sole_admin: {
				removal: {
					kind: "ready",
					soleAdmin: true,
					teams: [],
					member: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						accountId: "00000000-0000-4000-8000-000000000301",
						role: "admin",
						source: "manual",
						displayName: "Rae Okafor",
						email: "rae@meridian.co",
						joinedAt: "2026-01-05T09:00:00Z",
						lastActiveAt: "2026-08-02T09:00:00Z",
						lastAuthMethod: "password",
					},
				},
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000301",
							role: "admin",
							source: "manual",
							displayName: "Rae Okafor",
							email: "rae@meridian.co",
							joinedAt: "2026-01-05T09:00:00Z",
							lastActiveAt: "2026-08-02T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removal_not_in_view: {
				removal: { kind: "not_in_view", accountId: "00000000-0000-4000-8000-000000000309" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removal_unavailable: {
				removal: { kind: "unavailable", accountId: "00000000-0000-4000-8000-000000000302" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removing: {
				removal: {
					kind: "removing",
					soleAdmin: false,
					teams: [],
					member: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						accountId: "00000000-0000-4000-8000-000000000302",
						role: "member",
						source: "manual",
						displayName: "Jun Watanabe",
						email: "jun@meridian.co",
						joinedAt: "2026-02-11T09:00:00Z",
						lastActiveAt: "2026-07-20T09:00:00Z",
						lastAuthMethod: "password",
					},
				},
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000302",
							role: "member",
							source: "manual",
							displayName: "Jun Watanabe",
							email: "jun@meridian.co",
							joinedAt: "2026-02-11T09:00:00Z",
							lastActiveAt: "2026-07-20T09:00:00Z",
							lastAuthMethod: "password",
						},
					],
				},
			},
			removed: {
				notice: { kind: "removed", name: "Jun Watanabe", reassigned: "Sana Ali" },
				listing: {
					kind: "results",
					members: [
						{
							workspaceId: "00000000-0000-4000-8000-000000000000",
							accountId: "00000000-0000-4000-8000-000000000304",
							role: "member",
							source: "manual",
							displayName: "Sana Ali",
							email: "sana@meridian.co",
							joinedAt: "2026-04-18T09:00:00Z",
						},
					],
				},
			},
		}
	: {};
