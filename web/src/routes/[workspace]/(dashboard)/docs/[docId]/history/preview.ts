import type { DocFailure, DocHistory } from "$lib/docs/docs";

export type DocHistoryPreview = { history: DocHistory; failure?: DocFailure; restoring?: boolean };

export const docHistoryPreviewStates: Record<string, DocHistoryPreview> = import.meta.env.DEV
	? {
			loading: { history: { kind: "loading" } },
			unavailable: { history: { kind: "unavailable" } },
			not_found: { history: { kind: "not_found" } },
			...Object.fromEntries(
				(["ready", "restoring", "stale", "read_only", "revision_unavailable"] as const).map((state) => [
					state,
					{
						history: {
							kind: "ready",
							doc: {
								id: "00000000-0000-4000-8000-000000000a01",
								title: "On-call runbook",
								audience: { visibility: "workspace" },
								authorAccountId: "00000000-0000-4000-8000-000000000201",
								authorName: "Rae Okafor",
								status: "active",
								version: 3,
								createdAt: "2026-09-01T09:00:00Z",
								updatedAt: "2026-10-20T09:00:00Z",
								markdown: "Acknowledge within five minutes, then open the incident channel.\n\nPage the payments lead.\n",
								content: { type: "doc", content: [] },
								contributorIds: [],
								viewer: { canEdit: state !== "read_only", canShare: state !== "read_only" },
							},
							agents: [
								{
									id: "00000000-0000-4000-8000-000000000c01",
									workspaceId: "00000000-0000-4000-8000-000000000001",
									accountId: "00000000-0000-4000-8000-000000000c11",
									ownerAccountId: "00000000-0000-4000-8000-000000000201",
									name: "Scribe",
									icon: "bot",
									status: "active",
									scope: "member",
									execution: "runner",
									actionLimit: 60,
									createdAt: "2026-08-01T09:00:00Z",
								},
							],
							revisions: [
								{
									id: "00000000-0000-4000-8000-000000000b03",
									docId: "00000000-0000-4000-8000-000000000a01",
									version: 3,
									action: "edited",
									title: "On-call runbook",
									audience: { visibility: "workspace" },
									source: "agent",
									agentId: "00000000-0000-4000-8000-000000000c01",
									accountName: "Rae Okafor",
									createdAt: "2026-10-20T09:00:00Z",
								},
								{
									id: "00000000-0000-4000-8000-000000000b02",
									docId: "00000000-0000-4000-8000-000000000a01",
									version: 2,
									action: "edited",
									title: "On-call runbook",
									audience: { visibility: "workspace" },
									source: "person",
									accountName: "Rae Okafor",
									createdAt: "2026-10-12T09:00:00Z",
								},
								{
									id: "00000000-0000-4000-8000-000000000b01",
									docId: "00000000-0000-4000-8000-000000000a01",
									version: 1,
									action: "created",
									title: "On-call runbook",
									audience: { visibility: "workspace" },
									source: "person",
									accountName: "Rae Okafor",
									createdAt: "2026-09-01T09:00:00Z",
								},
							],
							chosen:
								state === "revision_unavailable"
									? { kind: "unavailable" }
									: {
											revision: {
												id: "00000000-0000-4000-8000-000000000b02",
												docId: "00000000-0000-4000-8000-000000000a01",
												version: 2,
												action: "edited",
												title: "On-call runbook",
												audience: { visibility: "workspace" },
												source: "person",
												accountName: "Rae Okafor",
												createdAt: "2026-10-12T09:00:00Z",
												markdown: "Acknowledge within five minutes, then open the incident channel.\n",
												content: {
													type: "doc",
													content: [
														{
															type: "paragraph",
															content: [
																{
																	type: "text",
																	text: "Acknowledge within five minutes, then open the incident channel.",
																},
															],
														},
													],
												},
											},
											before: {
												id: "00000000-0000-4000-8000-000000000b01",
												docId: "00000000-0000-4000-8000-000000000a01",
												version: 1,
												action: "created",
												title: "On-call runbook",
												audience: { visibility: "workspace" },
												source: "person",
												accountName: "Rae Okafor",
												createdAt: "2026-09-01T09:00:00Z",
												markdown: "Restart the worker when paged.\n",
												content: { type: "doc", content: [] },
											},
										},
						},
						...(state === "restoring" ? { restoring: true } : {}),
						...(state === "stale" ? { failure: { kind: "stale" } } : {}),
					} satisfies DocHistoryPreview,
				])
			),
		}
	: {};
