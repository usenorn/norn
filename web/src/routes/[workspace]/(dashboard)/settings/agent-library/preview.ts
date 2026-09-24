import type {
	AgentLibraryListing,
	CapabilityDialogPreview,
	McpConnectOutcome,
} from "$lib/agents/agent-capabilities";
import type { MembershipRole } from "$lib/workspace/members";

export type AgentLibraryPreview = {
	listing?: AgentLibraryListing;
	agentNames?: Record<string, string>;
	outcome?: McpConnectOutcome;
	role?: MembershipRole;
	dialog?: CapabilityDialogPreview;
};

export const agentLibraryPreviewStates: Record<string, AgentLibraryPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" }, role: "admin" },
			empty: { listing: { kind: "empty" }, role: "admin" },
			ready: {
				role: "admin",
				agentNames: {
					"00000000-0000-4000-8000-0000000009c1": "triage-bot",
					"00000000-0000-4000-8000-0000000009c2": "release-notes",
				},
				listing: {
					kind: "ready",
					library: {
						skills: [
							{
								skill: {
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
								agentIds: ["00000000-0000-4000-8000-0000000009c1", "00000000-0000-4000-8000-0000000009c2"],
							},
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
						mcpServers: [
							{
								server: {
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
								agentIds: ["00000000-0000-4000-8000-0000000009c1"],
							},
							{
								server: {
									id: "00000000-0000-4000-8000-000000000c06",
									name: "github",
									transport: "http",
									command: "",
									args: [],
									url: "https://api.githubcopilot.com/mcp/",
									auth: "oauth",
									envKeys: [],
									headerKeys: [],
									oauthClientId: "Iv23liNorthwindApp",
									library: true,
									createdAt: "2026-09-23T09:00:00Z",
									updatedAt: "2026-09-23T09:00:00Z",
								},
								agentIds: [],
							},
						],
					},
				},
			},
			member: { listing: { kind: "empty" }, role: "member" },
			forbidden: { listing: { kind: "forbidden" }, role: "member" },
			unavailable: { listing: { kind: "unavailable" }, role: "admin" },
			signed_in: { listing: { kind: "empty" }, role: "admin", outcome: { kind: "connected" } },
			sign_in_refused: { listing: { kind: "empty" }, role: "admin", outcome: { kind: "refused" } },
			sign_in_failed: {
				listing: { kind: "empty" },
				role: "admin",
				outcome: { kind: "failed", reference: "01J8Z6Q4M2V5" },
			},
			add_skill: { listing: { kind: "empty" }, role: "admin", dialog: "skill" },
			add_mcp: { listing: { kind: "empty" }, role: "admin", dialog: "mcp" },
		}
	: {};
