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
							repository: "northwind/api",
							tokenSet: true,
							tokenHint: "9f2c",
							identityLogin: "northwind-bot",
							hookInstalled: true,
							mirrorLabel: "norn",
							status: "connected",
							verifiedAt: "2026-08-07T09:14:00Z",
							lastSeenAt: "2026-08-07T09:40:00Z",
							createdAt: "2026-07-02T10:00:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
						links: [],
					},
				},
				broken: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							repository: "northwind/api",
							tokenSet: true,
							tokenHint: "9f2c",
							hookInstalled: true,
							mirrorLabel: "norn",
							status: "broken",
							brokenReason: "credentials_rejected",
							brokenDetail: "Bad credentials",
							brokenAt: "2026-08-07T06:30:00Z",
							verifiedAt: "2026-08-06T22:10:00Z",
							createdAt: "2026-07-02T10:00:00Z",
							updatedAt: "2026-08-07T06:30:00Z",
						},
						links: [],
					},
				},
				hook_missing: {
					view: {
						kind: "detail",
						connection: {
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "gitlab",
							baseUrl: "https://gitlab.northwind.example",
							repository: "northwind/platform/billing",
							tokenSet: true,
							tokenHint: "31bd",
							hookInstalled: false,
							mirrorLabel: "norn",
							status: "connected",
							verifiedAt: "2026-08-07T08:02:00Z",
							createdAt: "2026-07-19T12:00:00Z",
							updatedAt: "2026-08-07T08:02:00Z",
						},
						links: [],
					},
				},
				not_found: { view: { kind: "not_found" } },
				forbidden: { view: { kind: "forbidden" } },
				unavailable: { view: { kind: "unavailable" } },
			}
		: {};
