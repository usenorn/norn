import type {
	MintedRepository,
	SourceControlDetailView,
	SourceControlFailure,
	SourceControlView,
} from "$lib/source-control/source-control";

export type SourceControlPreview = {
	view?: SourceControlView;
	failure?: SourceControlFailure;
	minted?: MintedRepository;
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
						{
							id: "00000000-0000-4000-8000-0000000000a2",
							provider: "gitlab",
							baseUrl: "https://gitlab.northwind.example",
							label: "platform deploy key",
							tokenSet: true,
							tokenHint: "31bd",
							repositoryCount: 1,
							status: "connected",
							verifiedAt: "2026-08-07T08:02:00Z",
							createdAt: "2026-07-19T12:00:00Z",
							updatedAt: "2026-08-07T08:02:00Z",
						},
					],
					repositories: [
						{
							id: "00000000-0000-4000-8000-0000000000b1",
							connectionId: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							fullName: "northwind/api",
							defaultBranch: "main",
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
							hookInstalled: true,
							routeCount: 0,
							createdAt: "2026-07-11T10:04:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
					],
				},
			},
			self_hosted: {
				view: {
					kind: "list",
					connections: [
						{
							id: "00000000-0000-4000-8000-0000000000a4",
							provider: "gitea",
							baseUrl: "https://git.internal.northwind.example",
							label: "internal forge",
							tokenSet: true,
							tokenHint: "4d1a",
							identityLogin: "norn-bot",
							repositoryCount: 1,
							allowPrivateAddress: true,
							caCertificateSet: true,
							capabilities: [
								"webhooks",
								"reviews",
								"changed_paths",
								"issues",
								"labels",
								"assignees",
							],
							missingCapabilities: ["checks"],
							status: "connected",
							verifiedAt: "2026-08-07T19:10:00Z",
							createdAt: "2026-08-07T18:55:00Z",
							updatedAt: "2026-08-07T19:10:00Z",
						},
					],
					repositories: [
						{
							id: "00000000-0000-4000-8000-0000000000b4",
							connectionId: "00000000-0000-4000-8000-0000000000a4",
							provider: "gitea",
							fullName: "platform/api",
							mirrorLabel: "norn",
							syncDirection: "both",
							webhooksDisabled: true,
							pollIntervalSeconds: 120,
							hookInstalled: false,
							routeCount: 1,
							createdAt: "2026-08-07T18:56:00Z",
							updatedAt: "2026-08-07T19:10:00Z",
						},
					],
				},
			},
			hook_missing: {
				view: {
					kind: "list",
					connections: [
						{
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
					],
					repositories: [
						{
							id: "00000000-0000-4000-8000-0000000000b1",
							connectionId: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							fullName: "northwind/api",
							mirrorLabel: "norn",
							pollIntervalSeconds: 300,
							hookInstalled: false,
							routeCount: 1,
							createdAt: "2026-07-02T10:04:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
					],
				},
			},
			broken: {
				view: {
					kind: "list",
					connections: [
						{
							id: "00000000-0000-4000-8000-0000000000a3",
							provider: "github",
							label: "northwind-bot",
							tokenSet: true,
							tokenHint: "77ab",
							repositoryCount: 1,
							status: "broken",
							brokenReason: "credentials_rejected",
							brokenDetail: "the token was refused",
							brokenAt: "2026-08-07T07:31:00Z",
							createdAt: "2026-06-11T09:00:00Z",
							updatedAt: "2026-08-07T07:31:00Z",
						},
					],
					repositories: [],
				},
			},
			minted: {
				view: {
					kind: "list",
					connections: [
						{
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
					],
					repositories: [
						{
							id: "00000000-0000-4000-8000-0000000000b1",
							connectionId: "00000000-0000-4000-8000-0000000000a1",
							provider: "github",
							fullName: "northwind/api",
							mirrorLabel: "norn",
							pollIntervalSeconds: 300,
							hookInstalled: true,
							routeCount: 0,
							createdAt: "2026-08-07T09:14:00Z",
							updatedAt: "2026-08-07T09:14:00Z",
						},
					],
				},
				minted: {
					repository: {
						id: "00000000-0000-4000-8000-0000000000b1",
						connectionId: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						fullName: "northwind/api",
						mirrorLabel: "norn",
						pollIntervalSeconds: 300,
						hookInstalled: true,
						routeCount: 0,
						createdAt: "2026-08-07T09:14:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					webhookUrl: "https://norn.northwind.example/v1/source-control/github/00000000-0000-4000-8000-0000000000b1",
					webhookSecret: "nrnscm_HiVvJq2S1kXo0bT9wPq7ZmF4cRa6LdN8",
				},
			},
			forbidden: { view: { kind: "forbidden" } },
			sealing_unavailable: { view: { kind: "sealing_unavailable" } },
			unavailable: { view: { kind: "unavailable" } },
			credentials_rejected: {
				view: { kind: "empty" },
				failure: { kind: "credentials_rejected" },
			},
			repository_unreachable: {
				view: { kind: "empty" },
				failure: { kind: "repository_unreachable" },
			},
			rate_limited: { view: { kind: "empty" }, failure: { kind: "rate_limited" } },
		}
	: {};
