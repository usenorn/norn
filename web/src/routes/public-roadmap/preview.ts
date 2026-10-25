import type { PublicRoadmapView } from "#lib/roadmap/public-roadmap.js";

export const publicRoadmapPreviewStates: Record<string, PublicRoadmapView> = import.meta.env.DEV
	? {
			loading: { kind: "loading" },
			unavailable: { kind: "unavailable" },
			failed: { kind: "failed", reference: "5f1c8d2e-7a4b-4c3e-9d10-roadmapfail01" },
			empty: {
				kind: "empty",
				roadmap: {
					title: "Northwind roadmap",
					description: "What we are building next for Northwind customers.",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [],
					issues: [],
				},
			},
			ready: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					description:
						"What we are building next for Northwind customers.\nDates move as we learn; this page always shows the latest plan.",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [
						{
							id: "nwpreviewcheckou",
							kind: "project",
							title: "Faster checkout",
							description: "Pay in two taps, with saved cards and wallets.",
							status: "in_progress",
							timeline: { startOn: "2026-09-01", endOn: "2026-11-30" },
							progress: 45,
							issues: [
								{ id: "nwpreviewwallets", kind: "issue", title: "Apple Pay and Google Pay" },
								{ id: "nwpreviewsavecrd", kind: "issue", title: "Saved cards" },
							],
						},
						{
							id: "nwpreviewreports",
							kind: "project",
							title: "Scheduled reports",
							description: "Weekly summaries delivered to your inbox.",
							status: "planned",
							timeline: { startOn: "2026-12-01", endOn: "2027-02-28" },
							progress: 0,
							issues: [],
						},
						{
							id: "nwpreviewmobilea",
							kind: "project",
							title: "Mobile app",
							status: "planned",
							timeline: { endOn: "2027-03-31" },
						},
					],
					issues: [
						{
							id: "nwpreviewdarkmod",
							kind: "issue",
							title: "Dark mode",
							description: "Follows your system setting.",
							status: "completed",
							timeline: { endOn: "2026-09-15" },
						},
					],
				},
			},
			projects_only: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [
						{ id: "nwpreviewcheckou", kind: "project", title: "Faster checkout" },
						{ id: "nwpreviewreports", kind: "project", title: "Scheduled reports" },
						{ id: "nwpreviewmobilea", kind: "project", title: "Mobile app" },
					],
					issues: [],
				},
			},
			no_dates: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [
						{
							id: "nwpreviewcheckou",
							kind: "project",
							title: "Faster checkout",
							status: "in_progress",
							progress: 45,
						},
						{ id: "nwpreviewreports", kind: "project", title: "Scheduled reports", status: "planned" },
					],
					issues: [],
				},
			},
			no_progress: {
				kind: "ready",
				roadmap: {
					title: "Northwind roadmap",
					publishedAt: "2026-10-01T09:00:00Z",
					projects: [
						{
							id: "nwpreviewcheckou",
							kind: "project",
							title: "Faster checkout",
							status: "in_progress",
							timeline: { startOn: "2026-09-01", endOn: "2026-11-30" },
						},
						{
							id: "nwpreviewreports",
							kind: "project",
							title: "Scheduled reports",
							status: "planned",
							timeline: { startOn: "2026-12-01", endOn: "2027-02-28" },
						},
					],
					issues: [],
				},
			},
		}
	: {};
