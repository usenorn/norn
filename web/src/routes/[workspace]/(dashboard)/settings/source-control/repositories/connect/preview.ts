import type { ConnectRepositoriesView } from "./+page.server";

export type ConnectRepositoriesPreview = { view: ConnectRepositoriesView };

/**
 * Guarded so nothing here is reachable in production, with every fixture inlined inside the
 * literal: a hoisted constant shared between entries survives tree-shaking even when the map
 * does not.
 */
export const connectRepositoriesPreviewStates: Record<string, ConnectRepositoriesPreview> =
	import.meta.env.DEV
		? {
				offered: {
					view: {
						kind: "choose",
						connections: [
							{
								id: "c1",
								provider: "github",
								authKind: "app",
								tokenSet: false,
								tokenHint: "",
								status: "connected",
								identityLogin: "nornbot[bot]",
								accountLogin: "northwind",
								verifiedAt: "2026-08-13T09:00:00Z",
								repositoryCount: 1,
								createdAt: "2026-08-11T09:00:00Z",
								updatedAt: "2026-08-13T09:00:00Z",
							},
						],
						chosen: {
							id: "c1",
							provider: "github",
							authKind: "app",
							tokenSet: false,
							tokenHint: "",
							status: "connected",
							identityLogin: "nornbot[bot]",
							accountLogin: "northwind",
							verifiedAt: "2026-08-13T09:00:00Z",
							repositoryCount: 1,
							createdAt: "2026-08-11T09:00:00Z",
							updatedAt: "2026-08-13T09:00:00Z",
						},
						offered: [
							{
								externalId: "1",
								fullName: "northwind/platform",
								private: true,
								defaultBranch: "main",
							},
							{ externalId: "2", fullName: "northwind/web", private: false, defaultBranch: "main" },
							{ externalId: "3", fullName: "northwind/docs", private: false, defaultBranch: "main" },
						],
						offerUnreadable: false,
						installUrl: "https://github.com/apps/norn-northwind/installations/new",
						connected: ["northwind/docs"],
					},
				},
				offer_unreadable: {
					view: {
						kind: "choose",
						connections: [
							{
								id: "c1",
								provider: "github",
								authKind: "app",
								tokenSet: false,
								tokenHint: "",
								status: "connected",
								identityLogin: "nornbot[bot]",
								accountLogin: "northwind",
								verifiedAt: "2026-08-13T09:00:00Z",
								repositoryCount: 0,
								createdAt: "2026-08-11T09:00:00Z",
								updatedAt: "2026-08-13T09:00:00Z",
							},
						],
						chosen: {
							id: "c1",
							provider: "github",
							authKind: "app",
							tokenSet: false,
							tokenHint: "",
							status: "connected",
							identityLogin: "nornbot[bot]",
							accountLogin: "northwind",
							verifiedAt: "2026-08-13T09:00:00Z",
							repositoryCount: 0,
							createdAt: "2026-08-11T09:00:00Z",
							updatedAt: "2026-08-13T09:00:00Z",
						},
						offered: [],
						offerUnreadable: true,
						installUrl: "https://github.com/apps/norn-northwind/installations/new",
						connected: [],
					},
				},
				granted_nothing: {
					view: {
						kind: "choose",
						connections: [
							{
								id: "c1",
								provider: "github",
								authKind: "app",
								tokenSet: false,
								tokenHint: "",
								status: "connected",
								identityLogin: "nornbot[bot]",
								accountLogin: "northwind",
								verifiedAt: "2026-08-13T09:00:00Z",
								repositoryCount: 0,
								createdAt: "2026-08-11T09:00:00Z",
								updatedAt: "2026-08-13T09:00:00Z",
							},
						],
						chosen: {
							id: "c1",
							provider: "github",
							authKind: "app",
							tokenSet: false,
							tokenHint: "",
							status: "connected",
							identityLogin: "nornbot[bot]",
							accountLogin: "northwind",
							verifiedAt: "2026-08-13T09:00:00Z",
							repositoryCount: 0,
							createdAt: "2026-08-11T09:00:00Z",
							updatedAt: "2026-08-13T09:00:00Z",
						},
						offered: [],
						offerUnreadable: false,
						installUrl: "https://github.com/apps/norn-northwind/installations/new",
						connected: [],
					},
				},
				no_connection: { view: { kind: "no_connection" } },
				forbidden: { view: { kind: "forbidden" } },
				unavailable: { view: { kind: "unavailable" } },
			}
		: {};
