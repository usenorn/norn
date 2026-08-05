import type { ActivityFeed } from "$lib/activity/activity";
import type { WorkspaceAgent } from "$lib/agents/agents";

export type AgentRecordPreview = {
	agent?: WorkspaceAgent | null;
	activity?: ActivityFeed;
};

export const agentRecordPreviewStates: Record<string, AgentRecordPreview> = import.meta.env.DEV
	? {
			loading: {
				agent: {
					agent: {
						id: "00000000-0000-4000-8000-0000000009c1",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d1",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "triage-bot",
						status: "active",
						actionLimit: 120,
						createdAt: "2026-07-02T09:00:00Z",
					},
					ownerName: "Rae Chen",
					ownerEmail: "rae@northwind.co",
				},
				activity: { kind: "loading" },
			},
			empty: {
				agent: {
					agent: {
						id: "00000000-0000-4000-8000-0000000009c1",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d1",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "triage-bot",
						status: "active",
						actionLimit: 120,
						createdAt: "2026-07-02T09:00:00Z",
					},
					ownerName: "Rae Chen",
					ownerEmail: "rae@northwind.co",
				},
				activity: { kind: "empty" },
			},
			ready: {
				agent: {
					agent: {
						id: "00000000-0000-4000-8000-0000000009c1",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d1",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "triage-bot",
						status: "active",
						actionLimit: 120,
						createdAt: "2026-07-02T09:00:00Z",
					},
					ownerName: "Rae Chen",
					ownerEmail: "rae@northwind.co",
				},
				activity: {
					kind: "ready",
					events: [
						{
							id: "00000000-0000-4000-8000-0000000009f1",
							subjectKind: "issue",
							issueId: "00000000-0000-4000-8000-0000000009f9",
							actorAccountId: "00000000-0000-4000-8000-0000000009d1",
							actorName: "triage-bot",
							actorKind: "agent",
							createdAt: "2026-08-05T10:24:00Z",
							changes: [
								{
									id: "00000000-0000-4000-8000-000000000a01",
									kind: "property_changed",
									field: "priority",
									fromValue: "none",
									toValue: "urgent",
								},
							],
						},
						{
							id: "00000000-0000-4000-8000-0000000009f2",
							subjectKind: "issue",
							issueId: "00000000-0000-4000-8000-0000000009f8",
							actorAccountId: "00000000-0000-4000-8000-0000000009d1",
							actorName: "triage-bot",
							actorKind: "agent",
							createdAt: "2026-08-05T09:02:00Z",
							changes: [
								{
									id: "00000000-0000-4000-8000-000000000a02",
									kind: "state_changed",
									fromState: "Backlog",
									toState: "Todo",
								},
							],
						},
					],
				},
			},
			disabled: {
				agent: {
					agent: {
						id: "00000000-0000-4000-8000-0000000009c2",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d2",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "release-notes",
						status: "disabled",
						actionLimit: 30,
						disabledAt: "2026-08-01T11:00:00Z",
						createdAt: "2026-05-14T09:00:00Z",
					},
					ownerName: "Rae Chen",
					ownerEmail: "rae@northwind.co",
				},
				activity: { kind: "empty" },
			},
			missing: { agent: null, activity: { kind: "unavailable" } },
			unavailable: {
				agent: {
					agent: {
						id: "00000000-0000-4000-8000-0000000009c1",
						workspaceId: "00000000-0000-4000-8000-0000000009a1",
						accountId: "00000000-0000-4000-8000-0000000009d1",
						ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
						name: "triage-bot",
						status: "active",
						actionLimit: 120,
						createdAt: "2026-07-02T09:00:00Z",
					},
					ownerName: "Rae Chen",
					ownerEmail: "rae@northwind.co",
				},
				activity: { kind: "unavailable" },
			},
		}
	: {};
