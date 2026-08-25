import type { ActivityFeed } from "$lib/activity/activity";
import type {
	AgentCapabilities,
	AgentCapabilityKind,
} from "$lib/agents/agent-capabilities";
import type { AgentRecord } from "$lib/agents/agent-record";

export type AgentDetailTab = "overview" | "capabilities" | "activity";

export type AgentRecordPreview = {
	record?: AgentRecord;
	activity?: ActivityFeed;
	capabilities?: AgentCapabilities;
	tab?: AgentDetailTab;
	dialog?: AgentCapabilityKind;
};

export const agentRecordPreviewStates: Record<string, AgentRecordPreview> = import.meta.env.DEV
	? {
			loading: {
				record: { kind: "loading" },
				activity: { kind: "loading" },
				capabilities: { kind: "loading" },
			},
			empty: {
				record: {
					kind: "ready",
					value: {
						agent: {
							id: "00000000-0000-4000-8000-0000000009c1",
							workspaceId: "00000000-0000-4000-8000-0000000009a1",
							accountId: "00000000-0000-4000-8000-0000000009d1",
							ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
							name: "triage-bot",
							icon: "inbox",
							status: "active",
							actionLimit: 120,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
						authority: {
							scopes: ["issue:read", "issue:manage", "comment:read", "comment:manage"],
							allTeams: true,
							teamIds: [],
						},
					},
				},
				activity: { kind: "empty" },
				capabilities: { kind: "empty" },
			},
			ready: {
				record: {
					kind: "ready",
					value: {
						agent: {
							id: "00000000-0000-4000-8000-0000000009c1",
							workspaceId: "00000000-0000-4000-8000-0000000009a1",
							accountId: "00000000-0000-4000-8000-0000000009d1",
							ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
							name: "triage-bot",
							icon: "inbox",
							status: "active",
							actionLimit: 120,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
						authority: {
							scopes: ["issue:read", "issue:manage", "project:read", "comment:manage"],
							allTeams: false,
							teamIds: ["00000000-0000-4000-8000-0000000009b1"],
						},
					},
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
					],
				},
				capabilities: {
					kind: "ready",
					skills: [
						{
							kind: "skill",
							name: "Northwind triage",
							source: "github.com/northwind-labs/agent-skills/triage-preview",
						},
					],
					mcpServers: [
						{
							kind: "mcp",
							name: "Playwright",
							transport: "stdio",
							command: "npx",
							args: ["@playwright/mcp@latest"],
						},
						{
							kind: "mcp",
							name: "Northwind docs preview",
							transport: "remote",
							url: "https://mcp-preview.northwind.invalid/docs",
							auth: "bearer",
						},
					],
				},
			},
			disabled: {
				record: {
					kind: "ready",
					value: {
						agent: {
							id: "00000000-0000-4000-8000-0000000009c2",
							workspaceId: "00000000-0000-4000-8000-0000000009a1",
							accountId: "00000000-0000-4000-8000-0000000009d2",
							ownerAccountId: "00000000-0000-4000-8000-0000000009e1",
							name: "release-notes",
							icon: "pencil",
							status: "disabled",
							actionLimit: 30,
							disabledAt: "2026-08-01T11:00:00Z",
							createdAt: "2026-05-14T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
						authority: {
							scopes: ["issue:read", "comment:read"],
							allTeams: true,
							teamIds: [],
						},
					},
				},
				activity: { kind: "empty" },
				capabilities: { kind: "empty" },
			},
			missing: { record: { kind: "missing" }, activity: { kind: "unavailable" } },
			forbidden: { record: { kind: "forbidden" }, activity: { kind: "unavailable" } },
			authority_missing: {
				record: { kind: "authority_missing" },
				activity: { kind: "unavailable" },
			},
			unavailable: { record: { kind: "unavailable" }, activity: { kind: "unavailable" } },
			capability_loading: { capabilities: { kind: "loading" }, tab: "capabilities" },
			capability_empty: { capabilities: { kind: "empty" }, tab: "capabilities" },
			capability_unavailable: {
				capabilities: { kind: "unavailable" },
				tab: "capabilities",
			},
			add_skill: { capabilities: { kind: "empty" }, tab: "capabilities", dialog: "skill" },
			add_mcp: { capabilities: { kind: "empty" }, tab: "capabilities", dialog: "mcp" },
		}
	: {};
