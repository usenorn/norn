import type { ProposalQueue } from "$lib/agents/agents";

export type ApprovalsPreview = {
	queue?: ProposalQueue;
	busy?: boolean;
};

export const approvalsPreviewStates: Record<string, ApprovalsPreview> = import.meta.env.DEV
	? {
			loading: { queue: { kind: "loading" } },
			empty: { queue: { kind: "empty" } },
			ready: {
				queue: {
					kind: "ready",
					proposals: [
						{
							id: "00000000-0000-4000-8000-000000000b01",
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							issueId: "00000000-0000-4000-8000-0000000009f9",
							teamId: "00000000-0000-4000-8000-0000000009b1",
							action: "issue_edit",
							status: "pending",
							title: "Payment retries drop the idempotency key",
							createdAt: "2026-08-05T11:40:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000b02",
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							issueId: "00000000-0000-4000-8000-0000000009f8",
							teamId: "00000000-0000-4000-8000-0000000009b1",
							action: "comment",
							status: "pending",
							body: "This looks like a duplicate of the retry bug from March.",
							createdAt: "2026-08-05T11:12:00Z",
						},
					],
				},
			},
			deciding: {
				queue: {
					kind: "ready",
					proposals: [
						{
							id: "00000000-0000-4000-8000-000000000b03",
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							issueId: "00000000-0000-4000-8000-0000000009f7",
							teamId: "00000000-0000-4000-8000-0000000009b1",
							action: "state_change",
							status: "pending",
							createdAt: "2026-08-05T10:55:00Z",
						},
					],
				},
				busy: true,
			},
			held_close: {
				queue: {
					kind: "ready",
					proposals: [
						{
							id: "00000000-0000-4000-8000-000000000b05",
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							issueId: "00000000-0000-4000-8000-0000000009f9",
							issueReference: "ENG-42",
							issueTitle: "Payment retries drop the idempotency key",
							teamId: "00000000-0000-4000-8000-0000000009b1",
							action: "state_change",
							status: "pending",
							stateName: "Done",
							questions: [
								{
									id: "00000000-0000-4000-8000-000000000d01",
									issueId: "00000000-0000-4000-8000-0000000009f9",
									question: "Does a replayed webhook count as a retry for this rule?",
									default: "treat a replay as a retry",
									deadline: "2026-08-05T12:40:00Z",
									answered: false,
									expired: true,
									standing: "treat a replay as a retry",
									askedByName: "triage-bot",
									actorKind: "agent" as const,
									createdAt: "2026-08-05T11:40:00Z",
								},
							],
							createdAt: "2026-08-05T12:41:00Z",
						},
					],
				},
			},
			forbidden: { queue: { kind: "forbidden" } },
			unavailable: { queue: { kind: "unavailable" } },
		}
	: {};
