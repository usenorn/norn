import type { InboxFilter, InboxListing, NotificationFailure } from "$lib/notifications/notifications";

export type InboxPreview = {
	listing: InboxListing;
	filter?: InboxFilter;
	unread?: number;
	failure?: NotificationFailure;
};

export const inboxPreviewStates: Record<string, InboxPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			caught_up: { listing: { kind: "caught_up" }, unread: 0 },
			empty: { listing: { kind: "empty" }, filter: "all", unread: 0 },
			ready: {
				unread: 4,
				listing: {
					kind: "ready",
					notifications: [
						{
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-000000000a01",
							kind: "commented",
							reason: "mentioned",
							actorAccountId: "00000000-0000-4000-8000-000000000201",
							actorName: "Rae Okafor",
							actorKind: "user",
							title: "Payments retry twice on a timeout",
							reference: "ENG-412",
							teamKey: "ENG",
							unreadCount: 1,
							lastEventAt: "2026-08-05T09:41:00Z",
						},
						{
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-000000000a02",
							kind: "state_changed",
							reason: "following",
							actorAccountId: "00000000-0000-4000-8000-000000000202",
							actorName: "Deploy bot",
							actorKind: "agent",
							title: "Session cookie is dropped behind the proxy",
							reference: "ENG-398",
							teamKey: "ENG",
							unreadCount: 12,
							lastEventAt: "2026-08-05T08:12:00Z",
						},
						{
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-000000000a03",
							kind: "assigned",
							reason: "assigned",
							actorAccountId: "00000000-0000-4000-8000-000000000203",
							actorName: "Mina Adeyemi",
							actorKind: "user",
							title: "Invitation links expire a day early",
							reference: "MOB-77",
							teamKey: "MOB",
							unreadCount: 1,
							lastEventAt: "2026-08-04T17:03:00Z",
						},
						{
							subjectKind: "team",
							subjectId: "00000000-0000-4000-8000-000000000101",
							kind: "membership",
							reason: "membership",
							actorAccountId: "00000000-0000-4000-8000-000000000201",
							actorName: "Rae Okafor",
							actorKind: "user",
							title: "Mobile",
							teamKey: "MOB",
							unreadCount: 1,
							lastEventAt: "2026-08-04T11:20:00Z",
						},
					],
					nextCursor: "cmVhZC1tb3Jl",
				},
			},
			snoozed: {
				unread: 1,
				filter: "all",
				listing: {
					kind: "ready",
					notifications: [
						{
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-000000000a04",
							kind: "commented",
							reason: "following",
							actorAccountId: "00000000-0000-4000-8000-000000000204",
							actorName: "Priya Raman",
							actorKind: "user",
							title: "Search drops the last page of results",
							reference: "ENG-120",
							teamKey: "ENG",
							unreadCount: 0,
							lastEventAt: "2026-08-03T14:00:00Z",
							snoozedUntil: "2026-08-06T09:00:00Z",
						},
					],
				},
			},
			failed: {
				unread: 1,
				failure: { kind: "snooze_past" },
				listing: {
					kind: "ready",
					notifications: [
						{
							subjectKind: "issue",
							subjectId: "00000000-0000-4000-8000-000000000a05",
							kind: "commented",
							reason: "following",
							actorAccountId: "00000000-0000-4000-8000-000000000205",
							actorName: "Sam Whitfield",
							actorKind: "user",
							title: "Attachment upload stalls over 20MB",
							reference: "ENG-9",
							teamKey: "ENG",
							unreadCount: 2,
							lastEventAt: "2026-08-02T10:30:00Z",
						},
					],
				},
			},
		}
	: {};
