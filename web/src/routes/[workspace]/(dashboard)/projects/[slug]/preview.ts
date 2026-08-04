import type { IssueProgress } from "$lib/issues/board";
import type { ProjectDetail } from "$lib/projects/projects";

export type ProjectPreview = {
	detail: ProjectDetail;
	progress?: IssueProgress;
};

export const projectPreviewStates: Record<string, ProjectPreview> = import.meta.env.DEV
	? {
			loading: { detail: { kind: "loading" } },
			unavailable: { detail: { kind: "unavailable" } },
			not_found: { detail: { kind: "not_found" } },
			concealed: {
				detail: {
					kind: "ready",
					project: {
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
					members: [
						{
							projectId: "00000000-0000-4000-8000-000000000801",
							accountId: "00000000-0000-4000-8000-000000000201",
							displayName: "Rae Okafor",
							email: "rae@northwind.co",
							createdAt: "2026-06-01T09:00:00Z",
						},
					],
					updates: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							projectId: "00000000-0000-4000-8000-000000000801",
							authorAccountId: "00000000-0000-4000-8000-000000000201",
							authorName: "Rae Okafor",
							health: "at_risk",
							body: "Auth migration slipped a week; the rest is on track.",
							createdAt: "2026-08-02T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000902",
							projectId: "00000000-0000-4000-8000-000000000801",
							authorAccountId: "00000000-0000-4000-8000-000000000201",
							authorName: "Rae Okafor",
							health: "on_track",
							body: "Payment provider sandbox is wired up.",
							createdAt: "2026-07-26T09:00:00Z",
						},
					],
					issues: [
						{
							id: "00000000-0000-4000-8000-000000000601",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamKey: "MOB",
							referenceKey: "MOB",
							number: 141,
							reference: "MOB-141",
							title: "Fix avatar upload on slow connections",
							description: "",
							priority: "high",
							status: "active",
							version: 3,
							labels: [],
							stateEnteredAt: "2026-07-28T09:00:00Z",
							createdAt: "2026-07-20T09:00:00Z",
							state: {
								id: "00000000-0000-4000-8000-000000000301",
								name: "In progress",
								category: "active",
								position: 3,
							},
						},
						{
							id: "00000000-0000-4000-8000-000000000602",
							workspaceId: "00000000-0000-4000-8000-000000000001",
							teamId: "00000000-0000-4000-8000-000000000102",
							teamKey: "PLT",
							referenceKey: "PLT",
							number: 88,
							reference: "PLT-88",
							title: "Rotate the signing keys",
							description: "",
							priority: "medium",
							status: "active",
							version: 1,
							labels: [],
							stateEnteredAt: "2026-07-30T09:00:00Z",
							createdAt: "2026-07-22T09:00:00Z",
							state: {
								id: "00000000-0000-4000-8000-000000000402",
								name: "Ready",
								category: "not_started",
								position: 2,
							},
						},
					],
				},
				progress: { notStarted: 1, active: 1, complete: 6, abandoned: 0 },
			},
			archived: {
				detail: {
					kind: "ready",
					project: {
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
					members: [],
					updates: [],
					issues: [],
				},
				progress: { notStarted: 0, active: 0, complete: 4, abandoned: 3 },
			},
			no_status: {
				detail: {
					kind: "ready",
					project: {
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
					members: [],
					updates: [],
					issues: [],
				},
			},
		}
	: {};
