import type { AgentFailure, AgentListing } from "$lib/agents/agents";
import type { Team } from "$lib/team/teams";

export type AgentsPreview = {
	listing?: AgentListing;
	teams?: Team[];
	busy?: boolean;
	failure?: AgentFailure;
};

export const agentsPreviewStates: Record<string, AgentsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" } },
			member_empty: { listing: { kind: "empty" } },
			member_ready: {
				listing: {
					kind: "ready",
					agents: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000009c3",
								workspaceId: "00000000-0000-4000-8000-0000000009a1",
								accountId: "00000000-0000-4000-8000-0000000009d3",
								ownerAccountId: "00000000-0000-4000-8000-0000000009e2",
								name: "my-triage-bot",
								icon: "inbox",
								status: "active",
								actionLimit: 120,
								createdAt: "2026-08-11T09:00:00Z",
							},
							ownerName: "Jun Park",
							ownerEmail: "jun@northwind.co",
							authority: {
								scopes: ["issue:read", "notification:read"],
								allTeams: true,
								teamIds: [],
							},
						},
					],
				},
			},
			ready: {
				listing: {
					kind: "ready",
					agents: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000009c1",
								workspaceId: "00000000-0000-4000-8000-0000000009a1",
								accountId: "00000000-0000-4000-8000-0000000009d1",
								ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
								name: "triage-bot",
								icon: "search",
								status: "active",
								actionLimit: 120,
								createdAt: "2026-07-02T09:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
							authority: {
								scopes: ["issue:read", "issue:manage", "comment:read", "comment:manage"],
								allTeams: true,
								teamIds: [],
							},
						},
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000009c2",
								workspaceId: "00000000-0000-4000-8000-0000000009a1",
								accountId: "00000000-0000-4000-8000-0000000009d2",
								ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
								name: "release-notes",
								icon: "scroll-text",
								status: "disabled",
								actionLimit: 30,
								disabledAt: "2026-08-01T11:00:00Z",
								createdAt: "2026-05-14T09:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
							authority: {
								scopes: ["issue:read", "comment:read"],
								allTeams: false,
								teamIds: ["00000000-0000-4000-8000-0000000009f1"],
							},
						},
					],
				},
			},
			registered: {
				listing: {
					kind: "registered",
					value: "nrn_8fQ2mV6xR1pL4dK7hT0cB3nJ5sA9wY2uI7oP4kM1zX",
					agent: {
						id: "00000000-0000-4000-8000-0000000009c3",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d3",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "triage-bot",
						icon: "target",
						status: "active",
						actionLimit: 120,
						createdAt: "2026-08-05T12:00:00Z",
					},
					agents: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000009c3",
								workspaceId: "00000000-0000-4000-8000-0000000009a1",
								accountId: "00000000-0000-4000-8000-0000000009d3",
								ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
								name: "triage-bot",
								icon: "target",
								status: "active",
								actionLimit: 120,
								createdAt: "2026-08-05T12:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
							authority: {
								scopes: ["issue:read", "issue:manage", "comment:manage"],
								allTeams: true,
								teamIds: [],
							},
						},
					],
				},
			},
			registering: { listing: { kind: "empty" }, busy: true },
			action_failed: { listing: { kind: "empty" }, failure: { kind: "unavailable" } },
			forbidden: { listing: { kind: "forbidden" } },
			authority_missing: { listing: { kind: "authority_missing" } },
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
