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
			check_set: {
				queue: {
					kind: "ready",
					proposals: [
						{
							id: "00000000-0000-4000-8000-000000000b04",
							agentId: "00000000-0000-4000-8000-0000000009c1",
							agentName: "triage-bot",
							issueId: "00000000-0000-4000-8000-0000000009f9",
							issueReference: "ENG-42",
							issueTitle: "Payment retries drop the idempotency key",
							teamId: "00000000-0000-4000-8000-0000000009b1",
							action: "check_set",
							status: "pending",
							checkIds: ["00000000-0000-4000-8000-000000000c01"],
							reasoning: {
								observed: "the retry path rebuilds the request and loses the header",
								uncertain: "whether a replayed webhook should count as a retry",
							},
							proposedChecks: [
								{
									id: "00000000-0000-4000-8000-000000000c01",
									workspaceId: "00000000-0000-4000-8000-000000000000",
									issueId: "00000000-0000-4000-8000-0000000009f9",
									position: 0,
									statement: "a retried payment reuses the idempotency key",
									method: "command" as const,
									proof: "go test ./internal/service/payment/...",
									timeLimitSeconds: 2592000,
									approval: "pending" as const,
									resolution: "none" as const,
									authorKind: "agent" as const,
									addedAfterDelegation: true,
									state: "unproven" as const,
									blocking: false,
									restsOnAbsence: false,
									expired: false,
									evidenceCount: 0,
									createdAt: "2026-08-05T11:40:00Z",
									updatedAt: "2026-08-05T11:40:00Z",
								},
							],
							createdAt: "2026-08-05T11:40:00Z",
						},
					],
				},
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
							checkState: {
								summary: {
									total: 1,
									proven: 0,
									unproven: 1,
									failed: 0,
									waived: 0,
									gaps: 0,
									expired: 0,
									unapproved: 0,
									blocking: 1,
									restingOnAbsence: 0,
								},
								checks: [
									{
										id: "00000000-0000-4000-8000-000000000c02",
										workspaceId: "00000000-0000-4000-8000-000000000000",
										issueId: "00000000-0000-4000-8000-0000000009f9",
										position: 0,
										statement: "a retried payment reuses the idempotency key",
										method: "command" as const,
										proof: "go test ./internal/service/payment/...",
										timeLimitSeconds: 2592000,
										approval: "approved" as const,
										resolution: "none" as const,
										authorKind: "user" as const,
										addedAfterDelegation: false,
										state: "unproven" as const,
										blocking: true,
										restsOnAbsence: false,
										expired: false,
										evidenceCount: 0,
										createdAt: "2026-08-05T11:40:00Z",
										updatedAt: "2026-08-05T11:40:00Z",
									},
								],
							},
							createdAt: "2026-08-05T12:41:00Z",
						},
					],
				},
			},
			forbidden: { queue: { kind: "forbidden" } },
			unavailable: { queue: { kind: "unavailable" } },
		}
	: {};
