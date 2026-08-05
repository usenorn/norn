import type { AgentListing } from "$lib/agents/agents";
import type { components } from "$lib/api/dashboard.gen";
import type { Team } from "$lib/team/teams";

type Membership = components["schemas"]["Membership"];

export type AgentsPreview = {
	listing?: AgentListing;
	people?: Membership[];
	teams?: Team[];
	busy?: boolean;
};

export const agentsPreviewStates: Record<string, AgentsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" } },
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
								status: "active",
								actionLimit: 120,
								createdAt: "2026-07-02T09:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
						},
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000009c2",
								workspaceId: "00000000-0000-4000-8000-0000000009a1",
								accountId: "00000000-0000-4000-8000-0000000009d2",
								ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
								name: "release-notes",
								status: "disabled",
								actionLimit: 30,
								disabledAt: "2026-08-01T11:00:00Z",
								createdAt: "2026-05-14T09:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
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
								status: "active",
								actionLimit: 120,
								createdAt: "2026-08-05T12:00:00Z",
							},
							ownerName: "Rae Chen",
							ownerEmail: "rae@northwind.co",
						},
					],
				},
			},
			registering: { listing: { kind: "empty" }, busy: true },
			forbidden: { listing: { kind: "forbidden" } },
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
