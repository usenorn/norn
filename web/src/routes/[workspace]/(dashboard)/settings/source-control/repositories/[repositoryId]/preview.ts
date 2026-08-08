import type {
	SourceControlFailure,
	SourceControlRepositoryView,
} from "$lib/source-control/source-control";

export type SourceControlRepositoryPreview = {
	view?: SourceControlRepositoryView;
	failure?: SourceControlFailure;
};

export const sourceControlRepositoryPreviewStates: Record<
	string,
	SourceControlRepositoryPreview
> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			detail: {
				view: {
					kind: "detail",
					connection: {
						id: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						label: "northwind-bot",
						authKind: "token",
						tokenSet: true,
						tokenHint: "9f2c",
						status: "connected",
						verifiedAt: "2026-08-07T09:14:00Z",
						createdAt: "2026-07-02T10:00:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					repository: {
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
					routes: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							repositoryId: "00000000-0000-4000-8000-0000000000b1",
							teamId: "00000000-0000-4000-8000-0000000000d1",
							pathPrefix: "services/api",
							createdAt: "2026-07-02T10:06:00Z",
						},
						{
							id: "00000000-0000-4000-8000-0000000000c2",
							repositoryId: "00000000-0000-4000-8000-0000000000b1",
							teamId: "00000000-0000-4000-8000-0000000000d2",
							pathPrefix: "",
							createdAt: "2026-07-02T10:07:00Z",
						},
					],
					deliveries: [
						{
							id: "00000000-0000-4000-8000-0000000000e1",
							event: "pull_request",
							outcome: "applied",
							detail: "linked 1, advanced 1",
							attempt: 0,
							receivedAt: "2026-08-07T09:40:00Z",
							processedAt: "2026-08-07T09:40:01Z",
						},
						{
							id: "00000000-0000-4000-8000-0000000000e2",
							event: "push",
							outcome: "ignored",
							detail: "nothing in this delivery named an issue",
							attempt: 0,
							receivedAt: "2026-08-07T09:12:00Z",
							processedAt: "2026-08-07T09:12:01Z",
						},
					],
				},
			},
			unrouted: {
				view: {
					kind: "detail",
					connection: {
						id: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						label: "northwind-bot",
						authKind: "token",
						tokenSet: true,
						tokenHint: "9f2c",
						status: "connected",
						createdAt: "2026-07-02T10:00:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					repository: {
						id: "00000000-0000-4000-8000-0000000000b2",
						connectionId: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						fullName: "northwind/web",
						mirrorLabel: "norn",
						pollIntervalSeconds: 300,
						hookInstalled: false,
						routeCount: 0,
						createdAt: "2026-08-07T09:14:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					routes: [],
					deliveries: [],
				},
			},
			polling_only: {
				view: {
					kind: "detail",
					connection: {
						id: "00000000-0000-4000-8000-0000000000a4",
						provider: "gitea",
						baseUrl: "https://git.internal.northwind.example",
						label: "internal forge",
						authKind: "token",
						tokenSet: true,
						tokenHint: "4d1a",
						allowPrivateAddress: true,
						caCertificateSet: true,
						status: "connected",
						createdAt: "2026-08-07T18:55:00Z",
						updatedAt: "2026-08-07T19:10:00Z",
					},
					repository: {
						id: "00000000-0000-4000-8000-0000000000b4",
						connectionId: "00000000-0000-4000-8000-0000000000a4",
						provider: "gitea",
						fullName: "platform/api",
						mirrorLabel: "norn",
						syncDirection: "inbound",
						webhooksDisabled: true,
						pollIntervalSeconds: 120,
						hookInstalled: false,
						routeCount: 1,
						createdAt: "2026-08-07T18:56:00Z",
						updatedAt: "2026-08-07T19:10:00Z",
					},
					routes: [
						{
							id: "00000000-0000-4000-8000-0000000000c4",
							repositoryId: "00000000-0000-4000-8000-0000000000b4",
							teamId: "00000000-0000-4000-8000-0000000000d1",
							pathPrefix: "",
							createdAt: "2026-08-07T18:57:00Z",
						},
					],
					deliveries: [],
				},
			},
			not_found: { view: { kind: "not_found" } },
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			already_routed: {
				view: {
					kind: "detail",
					connection: {
						id: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						label: "northwind-bot",
						authKind: "token",
						tokenSet: true,
						tokenHint: "9f2c",
						status: "connected",
						createdAt: "2026-07-02T10:00:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					repository: {
						id: "00000000-0000-4000-8000-0000000000b1",
						connectionId: "00000000-0000-4000-8000-0000000000a1",
						provider: "github",
						fullName: "northwind/api",
						mirrorLabel: "norn",
						pollIntervalSeconds: 300,
						hookInstalled: true,
						routeCount: 1,
						createdAt: "2026-07-02T10:04:00Z",
						updatedAt: "2026-08-07T09:14:00Z",
					},
					routes: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							repositoryId: "00000000-0000-4000-8000-0000000000b1",
							teamId: "00000000-0000-4000-8000-0000000000d1",
							pathPrefix: "services/api",
							createdAt: "2026-07-02T10:06:00Z",
						},
					],
					deliveries: [],
				},
				failure: { kind: "already_routed" },
			},
		}
	: {};
