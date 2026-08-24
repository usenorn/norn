import type { ReviewQueue } from "$lib/executions/reviews";

export type ReviewsPreview = { queue: ReviewQueue };

export const reviewPreviewStates: Record<string, ReviewsPreview> = import.meta.env.DEV
	? {
			loading: { queue: { kind: "loading" } },
			unavailable: { queue: { kind: "unavailable" } },
			empty: { queue: { kind: "ready", runs: [] } },
			waiting: {
				queue: {
					kind: "ready",
					runs: [
						{
							execution: {
								id: "exec-01M0RCN1M3CGMBD9ZC12S4S5GF",
								reference: "DSG-14",
								workspaceId: "8f1c2f4a-6f2c-4a3e-9a1c-2b3d4e5f6071",
								issueId: "3a2b1c0d-9e8f-4a7b-8c6d-5e4f3a2b1c0d",
								issueReference: "DSG-14",
								issueTitle: "Median helper for the ledger",
								agentName: "Rae's agent",
								runnerName: "rae-mbp",
								codebaseName: "northwind",
								attempt: 1,
								state: "awaiting_review",
								params: { tool: "claude", model: "opus", permissionProfile: "standard" },
								queuedAt: "2026-08-24T09:02:00Z",
								startedAt: "2026-08-24T09:03:00Z",
								finishedAt: "2026-08-24T09:41:00Z",
							},
							change: {
								repositories: 2,
								commits: 5,
								additions: 1101,
								deletions: 89,
								filesChanged: 13,
								pullRequests: 2,
							},
						},
						{
							execution: {
								id: "exec-01M0RCN1M3CGMBD9ZC12S4S5GG",
								reference: "DSG-21-r2",
								workspaceId: "8f1c2f4a-6f2c-4a3e-9a1c-2b3d4e5f6071",
								issueId: "4b3c2d1e-0f9a-4b8c-9d7e-6f5a4b3c2d1e",
								issueReference: "DSG-21",
								issueTitle: "Stop the export from timing out on large ledgers",
								agentName: "Rae's agent",
								runnerName: "ci-box",
								codebaseName: "northwind",
								attempt: 2,
								state: "awaiting_review",
								params: { tool: "claude", model: "opus", permissionProfile: "standard" },
								queuedAt: "2026-08-24T08:10:00Z",
								startedAt: "2026-08-24T08:11:00Z",
								finishedAt: "2026-08-24T08:52:00Z",
							},
							change: {
								repositories: 1,
								commits: 2,
								additions: 41,
								deletions: 12,
								filesChanged: 3,
								pullRequests: 0,
							},
						},
					],
				},
			},
		}
	: {};
