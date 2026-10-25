import type { MemoryList } from "#lib/memories/memories.js";

export type MemoriesPreview = { list: MemoryList };

export const memoriesPreviewStates: Record<string, MemoriesPreview> = import.meta.env.DEV
	? {
			loading: { list: { kind: "loading" } },
			unavailable: { list: { kind: "unavailable" } },
			empty: { list: { kind: "empty" } },
			ready: {
				list: {
					kind: "ready",
					nextCursor: "preview",
					memories: [
						{
							id: "00000000-0000-4000-8000-000000000c01",
							kind: "decision",
							statement: "Production runs on MySQL 8",
							rationale: "The hosting contract bundles a managed MySQL cluster.",
							conditions: "all deployed environments",
							audience: { visibility: "workspace" },
							appliesTo: {},
							status: "disputed",
							basis: "inference",
							evidence: [],
							source: "agent",
							effectiveFrom: "2026-10-25",
							version: 1,
							contributorIds: [],
							viewer: { canEdit: true, canShare: true, canReview: false },
							createdAt: "2026-10-25T09:00:00Z",
							updatedAt: "2026-10-25T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000c02",
							kind: "constraint",
							statement: "Invoices are rounded half to even, never half up",
							rationale: "",
							conditions: "",
							audience: { visibility: "workspace" },
							appliesTo: { projectId: "00000000-0000-4000-8000-000000000b01", component: "ledger" },
							status: "proposed",
							basis: "observation",
							evidence: [],
							source: "person",
							effectiveFrom: "2026-10-24",
							version: 1,
							contributorIds: [],
							viewer: { canEdit: true, canShare: true, canReview: false },
							createdAt: "2026-10-24T15:30:00Z",
							updatedAt: "2026-10-24T15:30:00Z",
						},
					],
				},
			},
		}
	: {};
