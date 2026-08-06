import type { WebhookFailure, WebhooksView } from "$lib/webhooks/webhooks";

export type WebhooksPreview = {
	view?: WebhooksView;
	failure?: WebhookFailure;
	busy?: boolean;
};

export const webhooksPreviewStates: Record<string, WebhooksPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			empty: { view: { kind: "empty" } },
			list: {
				view: {
					kind: "list",
					webhooks: [
						{
							id: "00000000-0000-4000-8000-00000000c001",
							name: "Deploy pipeline",
							url: "https://hooks.northwind.example/norn/deploys",
							events: ["issue.created", "issue.status_changed", "issue.merged", "comment.posted"],
							secretHint: "whsec_…4f2a",
							enabled: true,
							failureStreak: 0,
							lastDeliveredAt: "2026-08-05T09:14:22Z",
							createdAt: "2026-06-02T10:00:00Z",
							updatedAt: "2026-08-05T09:14:22Z",
						},
						{
							id: "00000000-0000-4000-8000-00000000c002",
							name: "Status page",
							url: "https://hooks.northwind.example/norn/status",
							events: [
								"project.created",
								"project.updated",
								"project.status_posted",
								"cycle.started",
								"cycle.closed",
							],
							secretHint: "whsec_…91bd",
							enabled: true,
							failureStreak: 3,
							lastDeliveredAt: "2026-08-04T16:02:11Z",
							createdAt: "2026-05-19T08:30:00Z",
							updatedAt: "2026-08-04T16:02:11Z",
						},
						{
							id: "00000000-0000-4000-8000-00000000c003",
							name: "Roster mirror",
							url: "https://hooks.northwind.example/norn/roster",
							events: ["membership.added", "membership.removed"],
							secretHint: "whsec_…0c77",
							enabled: false,
							disabledAt: "2026-08-01T11:45:00Z",
							disabledReason: "sustained_failure",
							failureStreak: 12,
							createdAt: "2026-04-01T09:00:00Z",
							updatedAt: "2026-08-01T11:45:00Z",
						},
					],
				},
			},
			created: {
				view: {
					kind: "created",
					secret: "whsec_PreviewOnlyNeverShownAgain7Kq2",
					webhook: {
						id: "00000000-0000-4000-8000-00000000c004",
						name: "Deploy pipeline",
						url: "https://hooks.northwind.example/norn/deploys",
						events: ["issue.created", "issue.status_changed"],
						secretHint: "whsec_…7Kq2",
						enabled: true,
						failureStreak: 0,
						createdAt: "2026-08-05T09:20:00Z",
						updatedAt: "2026-08-05T09:20:00Z",
					},
					webhooks: [
						{
							id: "00000000-0000-4000-8000-00000000c004",
							name: "Deploy pipeline",
							url: "https://hooks.northwind.example/norn/deploys",
							events: ["issue.created", "issue.status_changed"],
							secretHint: "whsec_…7Kq2",
							enabled: true,
							failureStreak: 0,
							createdAt: "2026-08-05T09:20:00Z",
							updatedAt: "2026-08-05T09:20:00Z",
						},
					],
				},
			},
			signing_unavailable: { view: { kind: "signing_unavailable" } },
			destination_refused: {
				view: { kind: "empty" },
				failure: { kind: "destination_refused" },
			},
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			busy: {
				view: {
					kind: "list",
					webhooks: [
						{
							id: "00000000-0000-4000-8000-00000000c001",
							name: "Deploy pipeline",
							url: "https://hooks.northwind.example/norn/deploys",
							events: ["issue.created", "issue.status_changed"],
							secretHint: "whsec_…4f2a",
							enabled: true,
							failureStreak: 0,
							lastDeliveredAt: "2026-08-05T09:14:22Z",
							createdAt: "2026-06-02T10:00:00Z",
							updatedAt: "2026-08-05T09:14:22Z",
						},
					],
				},
				busy: true,
			},
		}
	: {};
