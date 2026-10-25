import type { MemoryDraft, MemoryFailure } from "#lib/memories/memories.js";

export type SupersedeMemoryPreview = { draft?: MemoryDraft; failure?: MemoryFailure; submitting?: boolean };

export const supersedeMemoryPreviewStates: Record<string, SupersedeMemoryPreview> = import.meta.env.DEV
	? {
			loading: { draft: { kind: "loading" } },
			unavailable: { draft: { kind: "unavailable" } },
			"not-found": { draft: { kind: "not_found" } },
			"read-only": {
				draft: {
					kind: "read_only",
					memory: {
						id: "00000000-0000-4000-8000-000000000d03",
						kind: "decision",
						statement: "Customer exports run nightly at 02:00 UTC",
						rationale: "",
						conditions: "",
						audience: { visibility: "workspace" },
						redacted: false,
						appliesTo: {},
						status: "accepted",
						basis: "observation",
						evidence: [],
						source: "person",
						effectiveFrom: "2026-10-20",
						version: 2,
						contributorIds: [],
						viewer: { canEdit: false, canShare: false, canReview: false },
						createdAt: "2026-10-20T09:00:00Z",
						updatedAt: "2026-10-21T09:00:00Z",
					},
				},
			},
			closed: {
				draft: {
					kind: "closed",
					memory: {
						id: "00000000-0000-4000-8000-000000000d05",
						kind: "decision",
						statement: "Feature branches deploy to the shared staging",
						rationale: "",
						conditions: "",
						audience: { visibility: "workspace" },
						redacted: false,
						appliesTo: {},
						status: "withdrawn",
						basis: "observation",
						evidence: [],
						source: "person",
						effectiveFrom: "2026-09-02",
						version: 3,
						contributorIds: [],
						viewer: { canEdit: true, canShare: true, canReview: false },
						createdAt: "2026-09-02T09:00:00Z",
						updatedAt: "2026-10-01T09:00:00Z",
					},
				},
			},
			submitting: { submitting: true },
			conflict: { failure: { kind: "stale", version: 4 } },
			invalid: { failure: { kind: "invalid" } },
			failed: { failure: { kind: "unavailable" } },
		}
	: {};
