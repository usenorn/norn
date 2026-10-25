import type { MemoryDetail } from "#lib/memories/memories.js";

export type MemoryPreview = { detail: MemoryDetail };

export const memoryPreviewStates: Record<string, MemoryPreview> = import.meta.env.DEV
	? {
			loading: {
				detail: {
					kind: "loading",
				},
			},
			unavailable: {
				detail: {
					kind: "unavailable",
				},
			},
			not_found: {
				detail: {
					kind: "not_found",
				},
			},
			duplicate: {
				detail: {
					kind: "ready",
					memory: {
						id: "00000000-0000-4000-8000-000000000c21",
						kind: "decision",
						statement: "Production runs on Postgres 17.",
						rationale: "Recovery is in the hosting contract.",
						conditions: "All deployed environments",
						audience: {
							visibility: "workspace",
						},
						appliesTo: {},
						status: "proposed",
						basis: "inference",
						evidence: [],
						authorName: "Ledger agent",
						source: "agent",
						effectiveFrom: "2026-10-25",
						version: 1,
						contributorIds: [],
						viewer: {
							canEdit: true,
							canShare: true,
							canReview: false,
						},
						createdAt: "2026-10-25T09:00:00Z",
						updatedAt: "2026-10-25T09:00:00Z",
					},
					related: [
						{
							memory: {
								id: "00000000-0000-4000-8000-000000000c11",
								kind: "decision",
								statement: "Production runs on PostgreSQL 17",
								rationale: "Point-in-time recovery is part of the hosting contract.",
								conditions: "all deployed environments",
								audience: {
									visibility: "workspace",
								},
								appliesTo: {},
								status: "accepted",
								basis: "observation",
								evidence: [],
								authorName: "Jun Park",
								source: "person",
								effectiveFrom: "2026-10-25",
								version: 1,
								contributorIds: [],
								viewer: {
									canEdit: true,
									canShare: true,
									canReview: false,
								},
								createdAt: "2026-10-25T09:00:00Z",
								updatedAt: "2026-10-25T09:00:00Z",
							},
							relation: "duplicate",
						},
					],
					choices: [
						{
							resolution: "independent",
						},
						{
							resolution: "update",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "supersede",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "no_change",
						},
					],
				},
			},
			conditional: {
				detail: {
					kind: "ready",
					memory: {
						id: "00000000-0000-4000-8000-000000000c31",
						kind: "decision",
						statement: "Run SQLite for the database",
						rationale: "Nobody should need a server to try an idea.",
						conditions: "local prototypes only",
						audience: {
							visibility: "workspace",
						},
						appliesTo: {},
						status: "proposed",
						basis: "observation",
						evidence: [],
						authorName: "Rae Okafor",
						source: "person",
						effectiveFrom: "2026-10-25",
						version: 1,
						contributorIds: [],
						viewer: {
							canEdit: true,
							canShare: true,
							canReview: false,
						},
						createdAt: "2026-10-25T09:00:00Z",
						updatedAt: "2026-10-25T09:00:00Z",
					},
					related: [
						{
							memory: {
								id: "00000000-0000-4000-8000-000000000c11",
								kind: "decision",
								statement: "Production runs on PostgreSQL 17",
								rationale: "Point-in-time recovery is part of the hosting contract.",
								conditions: "all deployed environments",
								audience: {
									visibility: "workspace",
								},
								appliesTo: {},
								status: "accepted",
								basis: "observation",
								evidence: [],
								authorName: "Jun Park",
								source: "person",
								effectiveFrom: "2026-10-25",
								version: 1,
								contributorIds: [],
								viewer: {
									canEdit: true,
									canShare: true,
									canReview: false,
								},
								createdAt: "2026-10-25T09:00:00Z",
								updatedAt: "2026-10-25T09:00:00Z",
							},
							relation: "conditional",
						},
					],
					choices: [
						{
							resolution: "independent",
						},
						{
							resolution: "update",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "supersede",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "no_change",
						},
					],
				},
			},
			disputed: {
				detail: {
					kind: "ready",
					memory: {
						id: "00000000-0000-4000-8000-000000000c41",
						kind: "decision",
						statement: "Production runs on MySQL 8",
						rationale: "The hosting contract bundles a managed MySQL cluster.",
						conditions: "all deployed environments",
						audience: {
							visibility: "workspace",
						},
						appliesTo: {},
						status: "disputed",
						basis: "inference",
						evidence: [],
						authorName: "Ledger agent",
						source: "agent",
						effectiveFrom: "2026-10-25",
						version: 1,
						contributorIds: [],
						viewer: {
							canEdit: true,
							canShare: true,
							canReview: false,
						},
						createdAt: "2026-10-25T09:00:00Z",
						updatedAt: "2026-10-25T09:00:00Z",
					},
					related: [
						{
							memory: {
								id: "00000000-0000-4000-8000-000000000c11",
								kind: "decision",
								statement: "Production runs on PostgreSQL 17",
								rationale: "Point-in-time recovery is part of the hosting contract.",
								conditions: "all deployed environments",
								audience: {
									visibility: "workspace",
								},
								appliesTo: {},
								status: "accepted",
								basis: "observation",
								evidence: [],
								authorName: "Jun Park",
								source: "person",
								effectiveFrom: "2026-10-25",
								version: 1,
								contributorIds: [],
								viewer: {
									canEdit: true,
									canShare: true,
									canReview: false,
								},
								createdAt: "2026-10-25T09:00:00Z",
								updatedAt: "2026-10-25T09:00:00Z",
							},
							relation: "conflict",
						},
					],
					choices: [
						{
							resolution: "independent",
						},
						{
							resolution: "update",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "supersede",
							targetId: "00000000-0000-4000-8000-000000000c11",
						},
						{
							resolution: "no_change",
						},
					],
				},
			},
			accepted: {
				detail: {
					kind: "ready",
					memory: {
						id: "00000000-0000-4000-8000-000000000c51",
						kind: "decision",
						statement: "Production runs on MySQL 8",
						rationale: "The hosting contract bundles a managed MySQL cluster.",
						conditions: "all deployed environments",
						audience: {
							visibility: "workspace",
						},
						appliesTo: {
							component: "database",
						},
						status: "accepted",
						basis: "observation",
						evidence: [],
						authorName: "Rae Okafor",
						source: "person",
						effectiveFrom: "2026-10-25",
						version: 1,
						contributorIds: [],
						viewer: {
							canEdit: true,
							canShare: true,
							canReview: false,
						},
						createdAt: "2026-10-25T09:00:00Z",
						updatedAt: "2026-10-25T09:00:00Z",
						supersedesId: "00000000-0000-4000-8000-000000000c11",
					},
					related: [],
					choices: [],
				},
			},
			read_only: {
				detail: {
					kind: "ready",
					memory: {
						id: "00000000-0000-4000-8000-000000000c61",
						kind: "decision",
						statement: "Production runs on MySQL 8",
						rationale: "",
						conditions: "all deployed environments",
						audience: {
							visibility: "workspace",
						},
						appliesTo: {},
						status: "disputed",
						basis: "inference",
						evidence: [],
						authorName: "Ledger agent",
						source: "agent",
						effectiveFrom: "2026-10-25",
						version: 1,
						contributorIds: [],
						viewer: {
							canEdit: false,
							canShare: false,
							canReview: false,
						},
						createdAt: "2026-10-25T09:00:00Z",
						updatedAt: "2026-10-25T09:00:00Z",
					},
					related: [
						{
							memory: {
								id: "00000000-0000-4000-8000-000000000c11",
								kind: "decision",
								statement: "Production runs on PostgreSQL 17",
								rationale: "Point-in-time recovery is part of the hosting contract.",
								conditions: "all deployed environments",
								audience: {
									visibility: "workspace",
								},
								appliesTo: {},
								status: "accepted",
								basis: "observation",
								evidence: [],
								authorName: "Jun Park",
								source: "person",
								effectiveFrom: "2026-10-25",
								version: 1,
								contributorIds: [],
								viewer: {
									canEdit: false,
									canShare: false,
									canReview: false,
								},
								createdAt: "2026-10-25T09:00:00Z",
								updatedAt: "2026-10-25T09:00:00Z",
							},
							relation: "conflict",
						},
					],
					choices: [],
				},
			},
		}
	: {};
