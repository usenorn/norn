import type {
	WebhookDeliveryDetail,
	WebhookDetailView,
	WebhookFailure,
} from "$lib/webhooks/webhooks";

export type WebhookDetailPreview = {
	view?: WebhookDetailView;
	failure?: WebhookFailure;
	expanded?: string;
	details?: Record<string, WebhookDeliveryDetail>;
	busy?: boolean;
};

export const webhookDetailPreviewStates: Record<string, WebhookDetailPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			detail: {
				view: {
					kind: "detail",
					webhook: {
						id: "00000000-0000-4000-8000-00000000c001",
						name: "Deploy pipeline",
						url: "https://hooks.northwind.example/norn/deploys",
						events: ["issue.created", "issue.status_changed", "issue.merged"],
						secretHint: "whsec_…4f2a",
						enabled: true,
						failureStreak: 0,
						lastDeliveredAt: "2026-08-05T09:14:22Z",
						createdAt: "2026-06-02T10:00:00Z",
						updatedAt: "2026-08-05T09:14:22Z",
					},
					nextCursor: "eyJhZnRlciI6IjIwMjYtMDgtMDQifQ",
					deliveries: [
						{
							id: "00000000-0000-4000-8000-00000000d001",
							event: "issue.status_changed",
							state: "succeeded",
							attempt: 1,
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-00000000e001",
							settledAt: "2026-08-05T09:14:22Z",
							createdAt: "2026-08-05T09:14:21Z",
						},
						{
							id: "00000000-0000-4000-8000-00000000d002",
							event: "issue.created",
							state: "pending",
							attempt: 2,
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-00000000e002",
							nextAttemptAt: "2026-08-05T09:22:00Z",
							createdAt: "2026-08-05T09:12:04Z",
						},
						{
							id: "00000000-0000-4000-8000-00000000d003",
							event: "webhook.test",
							state: "failed",
							attempt: 6,
							settledAt: "2026-08-04T16:02:11Z",
							createdAt: "2026-08-04T15:30:00Z",
						},
					],
				},
			},
			disabled: {
				view: {
					kind: "detail",
					webhook: {
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
					lastAttempt: {
						attempt: 6,
						requestUrl: "https://hooks.northwind.example/norn/roster",
						resolvedAddress: "203.0.113.44:443",
						outcome: "rejected",
						statusCode: 502,
						responseExcerpt: "upstream connect error or disconnect/reset before headers",
						startedAt: "2026-08-01T11:44:58Z",
						finishedAt: "2026-08-01T11:45:00Z",
						elapsedMs: 2041,
					},
					deliveries: [
						{
							id: "00000000-0000-4000-8000-00000000d010",
							event: "membership.added",
							state: "failed",
							attempt: 6,
							subjectKind: "membership",
							subjectId: "00000000-0000-4000-8000-00000000e010",
							settledAt: "2026-08-01T11:45:00Z",
							createdAt: "2026-08-01T10:12:00Z",
						},
					],
				},
			},
			empty_log: {
				view: {
					kind: "detail",
					webhook: {
						id: "00000000-0000-4000-8000-00000000c005",
						name: "Status page",
						url: "https://hooks.northwind.example/norn/status",
						events: ["project.status_posted"],
						secretHint: "whsec_…91bd",
						enabled: true,
						failureStreak: 0,
						createdAt: "2026-08-05T09:20:00Z",
						updatedAt: "2026-08-05T09:20:00Z",
					},
					deliveries: [],
				},
			},
			rotated: {
				view: {
					kind: "detail",
					secret: "whsec_PreviewOnlyNeverShownAgain7Kq2",
					webhook: {
						id: "00000000-0000-4000-8000-00000000c001",
						name: "Deploy pipeline",
						url: "https://hooks.northwind.example/norn/deploys",
						events: ["issue.created", "issue.status_changed"],
						secretHint: "whsec_…7Kq2",
						enabled: true,
						failureStreak: 0,
						lastDeliveredAt: "2026-08-05T09:14:22Z",
						createdAt: "2026-06-02T10:00:00Z",
						updatedAt: "2026-08-05T09:30:00Z",
					},
					deliveries: [],
				},
			},
			expanded: {
				expanded: "00000000-0000-4000-8000-00000000d003",
				details: {
					"00000000-0000-4000-8000-00000000d003": {
						delivery: {
							id: "00000000-0000-4000-8000-00000000d003",
							event: "webhook.test",
							state: "failed",
							attempt: 2,
							settledAt: "2026-08-04T16:02:11Z",
							createdAt: "2026-08-04T15:30:00Z",
						},
						body: {
							event: "webhook.test",
							workspace: "northwind",
							deliveredAt: "2026-08-04T15:30:00Z",
						},
						attempts: [
							{
								attempt: 1,
								requestUrl: "https://hooks.northwind.example/norn/deploys",
								resolvedAddress: "203.0.113.44:443",
								outcome: "timed_out",
								error: "no response within 10s",
								startedAt: "2026-08-04T15:30:00Z",
								finishedAt: "2026-08-04T15:30:10Z",
								elapsedMs: 10000,
							},
							{
								attempt: 2,
								requestUrl: "https://hooks.northwind.example/norn/deploys",
								resolvedAddress: "203.0.113.44:443",
								outcome: "rejected",
								statusCode: 500,
								responseExcerpt: "{\"error\":\"handler panicked\"}",
								startedAt: "2026-08-04T16:02:09Z",
								finishedAt: "2026-08-04T16:02:11Z",
								elapsedMs: 1874,
							},
						],
					},
				},
				view: {
					kind: "detail",
					webhook: {
						id: "00000000-0000-4000-8000-00000000c001",
						name: "Deploy pipeline",
						url: "https://hooks.northwind.example/norn/deploys",
						events: ["issue.created", "issue.status_changed", "issue.merged"],
						secretHint: "whsec_…4f2a",
						enabled: true,
						failureStreak: 2,
						lastDeliveredAt: "2026-08-05T09:14:22Z",
						createdAt: "2026-06-02T10:00:00Z",
						updatedAt: "2026-08-05T09:14:22Z",
					},
					deliveries: [
						{
							id: "00000000-0000-4000-8000-00000000d003",
							event: "webhook.test",
							state: "failed",
							attempt: 2,
							settledAt: "2026-08-04T16:02:11Z",
							createdAt: "2026-08-04T15:30:00Z",
						},
					],
				},
			},
			not_found: { view: { kind: "not_found" } },
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
		}
	: {};
