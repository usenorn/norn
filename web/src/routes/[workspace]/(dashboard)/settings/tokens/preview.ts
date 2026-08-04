import type { TokenListing } from "$lib/workspace/tokens";

export type TokensPreview = {
	listing?: TokenListing;
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
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "CI pipeline",
							scopes: ["issue:read", "team:read"],
							createdAt: "2026-07-01T09:00:00Z",
							lastUsedAt: "2026-08-01T14:22:00Z",
							expiresAt: "2027-07-01T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000802",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Release bot",
							scopes: ["issue:manage"],
							createdAt: "2026-06-14T09:00:00Z",
						},
					],
				},
			},
			minted: {
				listing: {
					kind: "minted",
					value: "nrn_zk4p9w2mQ7xR3vB8nT1cD6fH0jL5sA2eY9uI4oP7kM",
					token: {
						id: "00000000-0000-4000-8000-000000000803",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						name: "Deploy key",
						scopes: ["issue:read"],
						createdAt: "2026-08-03T10:00:00Z",
					},
					tokens: [
						{
							id: "00000000-0000-4000-8000-000000000803",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Deploy key",
							scopes: ["issue:read"],
							createdAt: "2026-08-03T10:00:00Z",
						},
					],
				},
			},
			creating: { listing: { kind: "empty" }, busy: true },
			forbidden: { listing: { kind: "forbidden" } },
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
