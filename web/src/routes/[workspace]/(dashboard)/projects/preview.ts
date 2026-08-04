import type { ProjectListing } from "$lib/projects/projects";

export type ProjectsPreview = { listing: ProjectListing };

export const projectsPreviewStates: Record<string, ProjectsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			empty: { listing: { kind: "empty" } },
			ready: {
				listing: {
					kind: "ready",
					projects: [
						{
							id: "00000000-0000-4000-8000-000000000801",
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
							concealedWork: true,
							createdAt: "2026-06-01T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000802",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							slug: "offline-mode",
							name: "Mobile offline mode",
							description: "",
							state: "planned",
							archived: false,
							concealedWork: false,
							createdAt: "2026-07-14T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000803",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							slug: "billing-migration",
							name: "Billing migration",
							description: "",
							state: "completed",
							leadAccountId: "00000000-0000-4000-8000-000000000202",
							leadName: "Nadia Freeman",
							archived: false,
							health: "on_track",
							concealedWork: false,
							createdAt: "2026-02-10T09:00:00Z",
						},
					],
				},
			},
			archived: {
				listing: {
					kind: "ready",
					projects: [
						{
							id: "00000000-0000-4000-8000-000000000804",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							slug: "legacy-invoicing",
							name: "Legacy invoicing",
							description: "",
							state: "cancelled",
							archived: true,
							archivedAt: "2026-05-02T11:00:00Z",
							concealedWork: false,
							createdAt: "2026-01-08T09:00:00Z",
						},
					],
				},
			},
		}
	: {};
