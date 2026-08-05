import type { WorkspaceConnectionListing } from "$lib/account/connections";

export type WorkspaceConnectionsPreview = {
	listing?: WorkspaceConnectionListing;
	busy?: boolean;
};

export const workspaceConnectionsPreviewStates: Record<string, WorkspaceConnectionsPreview> =
	import.meta.env.DEV
		? {
				loading: { listing: { kind: "loading" } },
				empty: { listing: { kind: "empty" } },
				ready: {
					listing: {
						kind: "ready",
						connections: [
							{
								connection: {
									id: "00000000-0000-4000-8000-000000000901",
									clientName: "Claude",
									capability: "write",
									followsMembership: true,
									grants: [],
									createdAt: "2026-07-02T09:24:00Z",
									lastUsedAt: "2026-08-05T07:10:00Z",
								},
								ownerName: "Rae Ellison",
								ownerEmail: "rae@northwind.co",
							},
							{
								connection: {
									id: "00000000-0000-4000-8000-000000000902",
									clientName: "Issue digest bot",
									capability: "read",
									followsMembership: false,
									grants: [
										{ workspaceId: "00000000-0000-4000-8000-0000000009a1", allTeams: true },
									],
									createdAt: "2026-05-18T11:00:00Z",
								},
								ownerName: "Sam Ode",
								ownerEmail: "sam@northwind.co",
							},
						],
					},
				},
				revoking: {
					listing: {
						kind: "ready",
						connections: [
							{
								connection: {
									id: "00000000-0000-4000-8000-000000000901",
									clientName: "Claude",
									capability: "write",
									followsMembership: true,
									grants: [],
									createdAt: "2026-07-02T09:24:00Z",
								},
								ownerName: "Rae Ellison",
								ownerEmail: "rae@northwind.co",
							},
						],
					},
					busy: true,
				},
				forbidden: { listing: { kind: "forbidden" } },
				unavailable: { listing: { kind: "unavailable" } },
			}
		: {};
