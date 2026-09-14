import type { ProjectListing } from "$lib/projects/projects";

export type TeamProjectsPreview = { listing: ProjectListing };

export const teamProjectsPreviewStates: Record<string, TeamProjectsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			no_matches: { listing: { kind: "no_matches" } },
			ready: {
				listing: {
					kind: "ready",
					projects: [
						{
							id: "00000000-0000-4000-8000-000000000811",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							slug: "checkout-rebuild",
							name: "Checkout rebuild",
							description: "Replace the payment flow end to end.",
							state: "active",
							leadAccountId: "00000000-0000-4000-8000-000000000201",
							leadName: "Rae Okafor",
							targetOn: "2026-09-30",
							archived: false,
							health: "at_risk",
							concealedWork: false,
							createdAt: "2026-06-01T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000812",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							slug: "invoice-tax-lines",
							name: "Invoice tax lines",
							description: "",
							state: "planned",
							archived: false,
							concealedWork: false,
							createdAt: "2026-07-14T09:00:00Z",
						},
					],
				},
			},
		}
	: {};
