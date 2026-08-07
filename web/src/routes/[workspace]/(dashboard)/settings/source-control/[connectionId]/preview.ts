import type { SourceControlDetailPreview } from "../preview";

export const sourceControlDetailPreviewStates: Record<string, SourceControlDetailPreview> =
	import.meta.env.DEV
		? {
				loading: { view: { kind: "loading" } },
				detail: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							label: "northwind-bot",
							tokenSet: true,
							tokenHint: "9f2c",
							identityLogin: "northwind-bot",
							repositoryCount: 2,
							status: "connected",
							verifiedAt: "2026-08-07T09:14:00Z",
							createdAt: "2026-07-02T10:00:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
						repositories: [
							{
								id: "00000000-0000-4000-8000-0000000000b1",
								connectionId: "00000000-0000-4000-8000-0000000000a1",
								provider: "github",
								fullName: "northwind/api",
								mirrorLabel: "norn",
								pollIntervalSeconds: 300,
								hookInstalled: true,
								routeCount: 2,
								createdAt: "2026-07-02T10:04:00Z",
								updatedAt: "2026-08-07T09:14:00Z",
							},
							{
								id: "00000000-0000-4000-8000-0000000000b2",
								connectionId: "00000000-0000-4000-8000-0000000000a1",
								provider: "github",
								fullName: "northwind/web",
								mirrorLabel: "norn",
								pollIntervalSeconds: 300,
								hookInstalled: false,
								routeCount: 0,
								createdAt: "2026-07-11T10:04:00Z",
								updatedAt: "2026-08-07T09:14:00Z",
							},
						],
					},
				},
				no_repositories: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "gitlab",
							baseUrl: "https://gitlab.northwind.example",
							label: "platform deploy key",
							tokenSet: true,
							tokenHint: "31bd",
							repositoryCount: 0,
							status: "connected",
							verifiedAt: "2026-08-07T08:02:00Z",
							createdAt: "2026-07-19T12:00:00Z",
							updatedAt: "2026-08-07T08:02:00Z",
						},
						repositories: [],
					},
				},
				broken: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a3",
							provider: "github",
							label: "northwind-bot",
							tokenSet: true,
							tokenHint: "77ab",
							repositoryCount: 1,
							status: "broken",
							brokenReason: "credentials_rejected",
							brokenDetail: "The token was refused by the platform.",
							brokenAt: "2026-08-07T07:31:00Z",
							createdAt: "2026-06-11T09:00:00Z",
							updatedAt: "2026-08-07T07:31:00Z",
						},
						repositories: [],
					},
				},
				not_found: { view: { kind: "not_found" } },
				forbidden: { view: { kind: "forbidden" } },
				unavailable: { view: { kind: "unavailable" } },
				rate_limited: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							label: "northwind-bot",
							tokenSet: true,
							tokenHint: "9f2c",
							repositoryCount: 1,
							status: "connected",
							verifiedAt: "2026-08-07T09:14:00Z",
							createdAt: "2026-07-02T10:00:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
						repositories: [],
					},
					failure: { kind: "rate_limited" },
				},
			}
		: {};
