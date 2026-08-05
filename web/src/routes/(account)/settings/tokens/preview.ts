import type { TokenListing } from "$lib/account/tokens";
import type { components } from "$lib/api/dashboard.gen";

type WorkspaceSummary = components["schemas"]["Workspace"];

export type TokensPreview = {
	listing?: TokenListing;
	workspaces?: WorkspaceSummary[];
	busy?: boolean;
};

export const tokensPreviewStates: Record<string, TokensPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" } },
			ready: {
				listing: {
					kind: "ready",
					tokens: [
						{
							id: "00000000-0000-4000-8000-000000000801",
							name: "CI pipeline",
							scopes: ["issue:read", "team:read"],
							grants: [
								{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true },
								{
									workspaceId: "00000000-0000-4000-8000-0000000009a2",
									allTeams: false,
									teamIds: ["00000000-0000-4000-8000-0000000009b1"],
								},
							],
							createdAt: "2026-03-12T09:24:00Z",
							lastUsedAt: "2026-08-05T07:10:00Z",
							expiresAt: "2027-03-12T09:24:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000802",
							name: "Release bot",
							scopes: ["issue:manage"],
							grants: [{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true }],
							createdAt: "2026-06-01T11:00:00Z",
						},
					],
				},
			},
			expiring: {
				listing: {
					kind: "ready",
					tokens: [
						{
							id: "00000000-0000-4000-8000-000000000803",
							name: "Nightly export",
							scopes: ["issue:read"],
							grants: [{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true }],
							createdAt: "2025-08-20T09:00:00Z",
							lastUsedAt: "2026-08-05T02:00:00Z",
							expiresAt: "2026-08-11T09:00:00Z",
						},
					],
				},
			},
			minted: {
				listing: {
					kind: "minted",
					value: "nrn_zk4p9w2mQ7xR3vB8nT1cD6fH0jL5sA2eY9uI4oP7kM",
					token: {
						id: "00000000-0000-4000-8000-000000000804",
						name: "Deploy key",
						scopes: ["issue:read"],
						grants: [{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true }],
						createdAt: "2026-08-05T12:00:00Z",
						expiresAt: "2026-11-03T12:00:00Z",
					},
					tokens: [
						{
							id: "00000000-0000-4000-8000-000000000804",
							name: "Deploy key",
							scopes: ["issue:read"],
							grants: [{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true }],
							createdAt: "2026-08-05T12:00:00Z",
							expiresAt: "2026-11-03T12:00:00Z",
						},
					],
				},
			},
			creating: { listing: { kind: "empty" }, busy: true },
			forbidden: { listing: { kind: "forbidden" } },
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
