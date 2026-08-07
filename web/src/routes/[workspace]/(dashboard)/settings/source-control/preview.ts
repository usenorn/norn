import type {
	SourceControlDetailView,
	SourceControlFailure,
	SourceControlView,
} from "$lib/source-control/source-control";

export type SourceControlPreview = {
	view?: SourceControlView;
	failure?: SourceControlFailure;
};

export type SourceControlDetailPreview = {
	view?: SourceControlDetailView;
	failure?: SourceControlFailure;
};

export const sourceControlPreviewStates: Record<string, SourceControlPreview> = import.meta.env
	.DEV
	? {
			loading: { view: { kind: "loading" } },
			empty: { view: { kind: "empty" } },
			list: {
				view: {
					kind: "list",
					connections: [
						{
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
						{
							id: "00000000-0000-4000-8000-0000000000a2",
							provider: "gitlab",
							baseUrl: "https://gitlab.northwind.example",
							repository: "northwind/platform/billing",
							tokenSet: true,
							tokenHint: "31bd",
							hookInstalled: true,
							mirrorLabel: "norn",
							status: "connected",
							verifiedAt: "2026-08-07T08:02:00Z",
							createdAt: "2026-07-19T12:00:00Z",
							updatedAt: "2026-08-07T08:02:00Z",
						},
					],
				},
			},
			broken: {
				view: {
					kind: "list",
					connections: [
						{
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
					],
				},
			},
			connected: {
				view: {
					kind: "connected",
					connections: [
						{
							id: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							repository: "northwind/api",
							tokenSet: true,
							tokenHint: "9f2c",
							hookInstalled: true,
							mirrorLabel: "norn",
							status: "connected",
							verifiedAt: "2026-08-07T09:14:00Z",
							createdAt: "2026-08-07T09:14:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
					],
					connection: {
						id: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						repository: "northwind/api",
						tokenSet: true,
						tokenHint: "9f2c",
						hookInstalled: true,
						mirrorLabel: "norn",
						status: "connected",
						verifiedAt: "2026-08-07T09:14:00Z",
						createdAt: "2026-08-07T09:14:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					webhookUrl:
						"https://norn.northwind.example/v1/source-control/github/00000000-0000-4000-8000-0000000000a1",
					webhookSecret: "nrnscm_S3Xm2rQpLd8vHt1KfZbW7cYn0aEu4jRg",
				},
			},
			sealing_unavailable: { view: { kind: "sealing_unavailable" } },
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			rate_limited: {
				view: { kind: "empty" },
				failure: { kind: "rate_limited" },
			},
		}
	: {};
