import type { SearchListing } from "$lib/search/search";

export type SearchPreview = { query: string; listing: SearchListing };

export const searchPreviewStates: Record<string, SearchPreview> = import.meta.env.DEV
	? {
			idle: { query: "", listing: { kind: "idle" } },
			searching: { query: "retry", listing: { kind: "searching" } },
			unavailable: { query: "retry", listing: { kind: "unavailable" } },
			no_matches: { query: "quokka", listing: { kind: "no_matches", fuzzy: false } },
			fuzzy_no_matches: { query: "quokka", listing: { kind: "no_matches", fuzzy: true } },
			results: {
				query: "retry",
				listing: {
					kind: "results",
					fuzzy: false,
					groups: [
						{
							kind: "issue",
							more: true,
							results: [
								{
									kind: "issue",
									id: "00000000-0000-4000-8000-0000000000a1",
									issueId: "00000000-0000-4000-8000-0000000000a1",
									title: "Payments retry twice on a timeout",
									reference: "ENG-412",
									teamKey: "ENG",
									status: "active",
									titleHit: true,
									updatedAt: "2026-08-05T09:41:00Z",
								},
								{
									kind: "issue",
									id: "00000000-0000-4000-8000-0000000000a2",
									issueId: "00000000-0000-4000-8000-0000000000a2",
									title: "Backoff schedule is too aggressive",
									excerpt: "the retry window doubles on every attempt",
									reference: "ENG-398",
									teamKey: "ENG",
									status: "active",
									titleHit: false,
									updatedAt: "2026-08-04T15:02:00Z",
								},
							],
						},
						{
							kind: "comment",
							more: false,
							results: [
								{
									kind: "comment",
									id: "00000000-0000-4000-8000-0000000000c1",
									issueId: "00000000-0000-4000-8000-0000000000a1",
									title: "Payments retry twice on a timeout",
									excerpt: "the retry loop fires twice when the gateway times out",
									reference: "ENG-412",
									teamKey: "ENG",
									status: "active",
									titleHit: false,
									updatedAt: "2026-08-05T09:12:00Z",
								},
							],
						},
						{
							kind: "project",
							more: false,
							results: [
								{
									kind: "project",
									id: "00000000-0000-4000-8000-0000000000b1",
									title: "Billing hardening",
									excerpt: "retire the retry hacks in checkout",
									slug: "billing-hardening",
									status: "active",
									titleHit: false,
									updatedAt: "2026-08-01T10:00:00Z",
								},
							],
						},
						{
							kind: "person",
							more: false,
							results: [
								{
									kind: "person",
									id: "00000000-0000-4000-8000-0000000000d1",
									title: "Retry Bot Owner",
									status: "member",
									titleHit: true,
									updatedAt: "2026-07-20T10:00:00Z",
								},
							],
						},
					],
				},
			},
			fuzzy_results: {
				query: "paymnets",
				listing: {
					kind: "results",
					fuzzy: true,
					groups: [
						{
							kind: "issue",
							more: false,
							results: [
								{
									kind: "issue",
									id: "00000000-0000-4000-8000-0000000000a1",
									issueId: "00000000-0000-4000-8000-0000000000a1",
									title: "Payments retry twice on a timeout",
									reference: "ENG-412",
									teamKey: "ENG",
									status: "active",
									titleHit: true,
									updatedAt: "2026-08-05T09:41:00Z",
								},
							],
						},
					],
				},
			},
		}
	: {};
