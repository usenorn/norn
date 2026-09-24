import type { ActivityFeed } from "$lib/activity/activity";
import type {
	AgentCapabilities,
	AgentLibraryListing,
	CapabilityDialogPreview,
	McpConnectOutcome,
} from "$lib/agents/agent-capabilities";
import type { AgentRecord } from "$lib/agents/agent-record";
import type { MembershipRole } from "$lib/workspace/members";

export type AgentDetailTab = "overview" | "capabilities" | "activity";

export type AgentRecordPreview = {
	record?: AgentRecord;
	activity?: ActivityFeed;
	capabilities?: AgentCapabilities;
	library?: AgentLibraryListing;
	outcome?: McpConnectOutcome;
	role?: MembershipRole;
	tab?: AgentDetailTab;
	dialog?: CapabilityDialogPreview;
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
							id: "00000000-0000-4000-8000-000000000b01",
							name: "house-style",
							description: "Writes issue comments and pull request titles in Northwind's house style.",
							source: "manual",
							instructions: "---\nname: house-style\ndescription: Writes in Northwind's house style.\n---\n\nShort sentences.\n",
							contentHash: "5c1f0b7a",
							sizeBytes: 812,
							fileCount: 1,
							library: true,
							createdAt: "2026-09-20T09:00:00Z",
							updatedAt: "2026-09-20T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000b02",
							name: "pdf",
							description: "Fills and reads PDF forms with the bundled scripts.",
							source: "github",
							origin: {
								repository: "anthropics/skills",
								path: "skills/pdf",
								ref: "",
								revision: "9f1c0de5b2a4c6e8f0a1b3c5d7e9f1a3b5c7d9e1",
							},
							instructions: "---\nname: pdf\ndescription: Fills PDF forms.\n---\n\nUse scripts/fill.py.\n",
							contentHash: "a44e21d3",
							sizeBytes: 48210,
							fileCount: 7,
							library: false,
							createdAt: "2026-09-21T14:30:00Z",
							updatedAt: "2026-09-21T14:30:00Z",
						},
					],
					mcpServers: [
						{
							id: "00000000-0000-4000-8000-000000000c01",
							name: "linear",
							transport: "http",
							command: "",
							args: [],
							url: "https://mcp.linear.app/mcp",
							auth: "oauth",
							envKeys: [],
							headerKeys: [],
							oauthClientId: "",
							registryName: "app.linear/linear",
							registryVersion: "1.0.0",
							library: true,
							connection: {
								status: "connected",
								issuer: "https://mcp.linear.app",
								scopes: ["read", "write"],
								expiresAt: "2026-09-24T18:00:00Z",
								connectedAt: "2026-09-22T10:00:00Z",
							},
							createdAt: "2026-09-22T09:58:00Z",
							updatedAt: "2026-09-22T10:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000c02",
							name: "sentry",
							transport: "http",
							command: "",
							args: [],
							url: "https://mcp.sentry.dev/mcp",
							auth: "oauth",
							envKeys: [],
							headerKeys: [],
							oauthClientId: "",
							library: false,
							createdAt: "2026-09-23T08:00:00Z",
							updatedAt: "2026-09-23T08:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000c03",
							name: "notion",
							transport: "http",
							command: "",
							args: [],
							url: "https://mcp.notion.com/mcp",
							auth: "oauth",
							envKeys: [],
							headerKeys: [],
							oauthClientId: "",
							library: false,
							connection: {
								status: "failed",
								issuer: "https://api.notion.com",
								scopes: [],
								failure: "refresh_rejected",
								connectedAt: "2026-09-01T08:00:00Z",
							},
							createdAt: "2026-09-01T07:58:00Z",
							updatedAt: "2026-09-23T06:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000c04",
							name: "playwright",
							transport: "stdio",
							command: "npx",
							args: ["-y", "@playwright/mcp@latest"],
							url: "",
							auth: "none",
							envKeys: ["PLAYWRIGHT_BROWSERS_PATH"],
							headerKeys: [],
							oauthClientId: "",
							library: false,
							createdAt: "2026-09-19T12:00:00Z",
							updatedAt: "2026-09-19T12:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000c05",
							name: "northwind-docs",
							transport: "sse",
							command: "",
							args: [],
							url: "https://mcp.northwind.co/docs/sse",
							auth: "headers",
							envKeys: [],
							headerKeys: ["X-Api-Key"],
							oauthClientId: "",
							library: false,
							createdAt: "2026-09-18T12:00:00Z",
							updatedAt: "2026-09-18T12:00:00Z",
						},
					],
				},
				library: {
					kind: "ready",
					library: {
						skills: [
							{
								skill: {
									id: "00000000-0000-4000-8000-000000000b03",
									name: "release-notes",
									description: "Writes release notes from merged pull requests.",
									source: "manual",
									instructions: "---\nname: release-notes\ndescription: Writes release notes.\n---\n\nGroup by area.\n",
									contentHash: "7d0e9a11",
									sizeBytes: 640,
									fileCount: 1,
									library: true,
									createdAt: "2026-09-10T09:00:00Z",
									updatedAt: "2026-09-10T09:00:00Z",
								},
								agentIds: [],
							},
						],
						mcpServers: [],
					},
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
			capability_empty: {
				capabilities: { kind: "empty" },
				library: { kind: "empty" },
				tab: "capabilities",
			},
			capability_forbidden: { capabilities: { kind: "forbidden" }, tab: "capabilities" },
			capability_unavailable: { capabilities: { kind: "unavailable" }, tab: "capabilities" },
			signed_in: { capabilities: { kind: "empty" }, tab: "capabilities", outcome: { kind: "connected" } },
			sign_in_refused: { capabilities: { kind: "empty" }, tab: "capabilities", outcome: { kind: "refused" } },
			sign_in_expired: { capabilities: { kind: "empty" }, tab: "capabilities", outcome: { kind: "expired" } },
			sign_in_removed: { capabilities: { kind: "empty" }, tab: "capabilities", outcome: { kind: "removed" } },
			sign_in_failed: {
				capabilities: { kind: "empty" },
				tab: "capabilities",
				outcome: { kind: "failed", reference: "01J8Z6Q4M2V5" },
			},
			add_skill: { capabilities: { kind: "empty" }, tab: "capabilities", dialog: "skill" },
			add_mcp: { capabilities: { kind: "empty" }, tab: "capabilities", dialog: "mcp" },
			library_skill: {
				capabilities: { kind: "empty" },
				library: {
					kind: "ready",
					library: {
						skills: [
							{
								skill: {
									id: "00000000-0000-4000-8000-000000000b04",
									name: "triage-rules",
									description: "Sorts incoming issues by team and severity.",
									source: "github",
									origin: {
										repository: "northwind-labs/agent-skills",
										path: "triage",
										ref: "main",
										revision: "3b7c9e1f2a4d6b8c0e1f3a5b7c9d1e3f5a7b9c1d",
									},
									instructions: "---\nname: triage-rules\ndescription: Sorts issues.\n---\n\nRead the team map.\n",
									contentHash: "c0ffee12",
									sizeBytes: 2048,
									fileCount: 2,
									library: true,
									createdAt: "2026-09-12T09:00:00Z",
									updatedAt: "2026-09-12T09:00:00Z",
								},
								agentIds: [],
							},
						],
						mcpServers: [],
					},
				},
				tab: "capabilities",
				dialog: "picker-skill",
			},
		}
	: {};
