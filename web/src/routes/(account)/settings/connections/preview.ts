import type { ConnectionListing } from "$lib/account/connections";
import type { components } from "$lib/api/dashboard.gen";

type WorkspaceSummary = components["schemas"]["Workspace"];

export type ConnectionsPreview = {
	listing?: ConnectionListing;
	workspaces?: WorkspaceSummary[];
	busy?: boolean;
};

export const connectionsPreviewStates: Record<string, ConnectionsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" } },
			ready: {
				listing: {
					kind: "ready",
					connections: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							clientName: "Claude",
							capability: "write",
							followsMembership: true,
							grants: [],
							createdAt: "2026-07-02T09:24:00Z",
							lastUsedAt: "2026-08-05T07:10:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000902",
							clientName: "Issue digest bot",
							capability: "read",
							followsMembership: false,
							grants: [
								{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true },
								{
									workspaceId: "00000000-0000-4000-8000-0000000009a2",
									allTeams: false,
									teamIds: ["00000000-0000-4000-8000-0000000009b1"],
								},
							],
							createdAt: "2026-05-18T11:00:00Z",
						},
					],
				},
			},
			narrowing: {
				listing: {
					kind: "ready",
					connections: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							clientName: "Claude",
							capability: "write",
							followsMembership: true,
							grants: [],
							createdAt: "2026-07-02T09:24:00Z",
						},
					],
				},
				busy: true,
			},
			forbidden: { listing: { kind: "forbidden" } },
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
