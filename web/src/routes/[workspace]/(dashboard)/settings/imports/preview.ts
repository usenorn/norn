import type { ImportFailure, ImportsView } from "$lib/imports/imports";

export type ImportsPreview = {
	view?: ImportsView;
	failure?: ImportFailure;
	busy?: boolean;
};

export const importsPreviewStates: Record<string, ImportsPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			sources: {
				view: {
					kind: "sources",
					sources: [{ kind: "linear" }, { kind: "csv" }],
					runs: [],
				},
			},
			sources_history: {
				view: {
					kind: "sources",
					sources: [{ kind: "linear" }, { kind: "csv" }],
					runs: [
						{
							id: "00000000-0000-4000-8000-0000000d1001",
							sourceKind: "linear",
							sourceLabel: "Northwind engineering",
							status: "imported",
							acknowledgeTriage: true,
							unknownReferences: "skip",
							sourceSecretSet: true,
							attempt: 1,
							staged: 4182,
							processed: 4182,
							stagedAt: "2026-07-28T09:41:00Z",
							mappedAt: "2026-07-28T10:02:00Z",
							startedAt: "2026-07-28T10:04:00Z",
							finishedAt: "2026-07-28T10:39:00Z",
							createdAt: "2026-07-28T09:31:00Z",
							updatedAt: "2026-07-28T10:39:00Z",
						},
						{
							id: "00000000-0000-4000-8000-0000000d1002",
							sourceKind: "csv",
							sourceLabel: "Support backlog 2024",
							status: "reverted",
							acknowledgeTriage: false,
							unknownReferences: "create",
							sourceSecretSet: false,
							attempt: 1,
							staged: 318,
							processed: 318,
							stagedAt: "2026-07-14T13:10:00Z",
							startedAt: "2026-07-14T13:22:00Z",
							finishedAt: "2026-07-14T13:26:00Z",
							revertedAt: "2026-07-14T15:02:00Z",
							createdAt: "2026-07-14T13:04:00Z",
							updatedAt: "2026-07-14T15:02:00Z",
						},
						{
							id: "00000000-0000-4000-8000-0000000d1003",
							sourceKind: "linear",
							status: "failed",
							phaseError: "the source stopped answering partway through the comments",
							acknowledgeTriage: false,
							unknownReferences: "fail",
							sourceSecretSet: true,
							attempt: 3,
							staged: 902,
							processed: 411,
							stagedAt: "2026-06-30T08:15:00Z",
							startedAt: "2026-06-30T08:31:00Z",
							finishedAt: "2026-06-30T08:44:00Z",
							createdAt: "2026-06-30T08:02:00Z",
							updatedAt: "2026-06-30T08:44:00Z",
						},
						{
							id: "00000000-0000-4000-8000-0000000d1004",
							sourceKind: "csv",
							sourceLabel: "Hardware requests",
							status: "draft",
							acknowledgeTriage: false,
							unknownReferences: "skip",
							sourceSecretSet: false,
							attempt: 0,
							staged: 0,
							processed: 0,
							createdAt: "2026-08-05T16:20:00Z",
							updatedAt: "2026-08-05T16:20:00Z",
						},
					],
					nextCursor: "cHJldmlldy1jdXJzb3I",
				},
			},
			starting: {
				view: {
					kind: "sources",
					sources: [{ kind: "linear" }, { kind: "csv" }],
					runs: [],
				},
				busy: true,
			},
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			encryption_unavailable: {
				view: {
					kind: "sources",
					sources: [{ kind: "linear" }, { kind: "csv" }],
					runs: [],
				},
				failure: { kind: "encryption_unavailable" },
			},
		}
	: {};
