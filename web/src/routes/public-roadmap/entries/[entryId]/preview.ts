import type { PublicRoadmapEntryView } from "$lib/roadmap/public-roadmap";

export const publicRoadmapEntryPreviewStates: Record<string, PublicRoadmapEntryView> = import.meta
	.env.DEV
	? {
			loading: { kind: "loading" },
			unavailable: { kind: "unavailable" },
			failed: { kind: "failed", reference: "5f1c8d2e-7a4b-4c3e-9d10-roadmapfail02" },
			with_issues: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [],
					issues: [],
				},
				item: {
					id: "nwpreviewcheckou",
					kind: "project",
					title: "Faster checkout",
					description:
						"Pay in two taps, with saved cards and wallets.\nWe are rolling this out country by country.",
					status: "in_progress",
					timeline: { startOn: "2026-09-01", endOn: "2026-11-30" },
					progress: 45,
					issues: [
						{
							id: "nwpreviewwallets",
							kind: "issue",
							title: "Apple Pay and Google Pay",
							description: "Both wallets, on web and mobile.",
							status: "in_progress",
							timeline: { startOn: "2026-09-01", endOn: "2026-10-15" },
						},
						{
							id: "nwpreviewsavecrd",
							kind: "issue",
							title: "Saved cards",
							status: "planned",
							timeline: { startOn: "2026-10-15", endOn: "2026-11-30" },
						},
						{ id: "nwpreviewreceipt", kind: "issue", title: "Email receipts", status: "planned" },
					],
				},
			},
			projects_only: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [],
					issues: [],
				},
				item: { id: "nwpreviewcheckou", kind: "project", title: "Faster checkout" },
			},
			no_issues: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [],
					issues: [],
				},
				item: {
					id: "nwpreviewreports",
					kind: "project",
					title: "Scheduled reports",
					status: "planned",
					progress: 0,
					issues: [],
				},
			},
			entry_issue: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [],
					issues: [],
				},
				item: {
					id: "nwpreviewdarkmod",
					kind: "issue",
					title: "Dark mode",
					description: "Follows your system setting.",
					status: "completed",
					timeline: { endOn: "2026-09-15" },
				},
			},
		}
	: {};
