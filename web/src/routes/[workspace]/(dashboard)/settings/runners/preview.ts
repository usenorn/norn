import type { WorkspaceAgent } from "$lib/agents/agents";
import type { RunnerFailure, RunnersView } from "$lib/runners/runners";

export type RunnersPreview = {
	view?: RunnersView;
	failure?: RunnerFailure;
	agents?: WorkspaceAgent[];
};

export const runnersPreviewStates: Record<string, RunnersPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			no_agents: { view: { kind: "no_agents" } },
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			agent_without_runner: {
				view: {
					kind: "empty",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [],
							codebases: [],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			agent_disabled: {
				view: {
					kind: "empty",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: true,
							},
							machines: [],
							codebases: [],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			ready: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			no_folder: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			never_connected: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
						},
							],
							codebases: [],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			offline: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-23T18:04:00Z",
							load: { connected: false, capacity: 0, used: 0, free: 0, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			paused: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							pausedAt: "2026-08-23T22:10:00Z",
							load: { connected: true, capacity: 2, used: 0, free: 2, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			busy: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 2, free: 0, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			disk_pressure: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: true },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			revoked: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "revoked",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-23T09:11:00Z",
							revokedAt: "2026-08-23T09:12:00Z",
						},
							],
							codebases: [],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			drift: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "drift",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			disconnected_folder: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "disconnected",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							disconnectedAt: "2026-08-23T11:45:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			folders_unreadable: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [],
							folders: "unreadable",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
			},
			refused: {
				view: {
					kind: "ready",
					groups: [
						{
							agent: {
								id: "00000000-0000-4000-8000-0000000000a1",
								name: "opsy",
								owner: "Rae Chen",
								disabled: false,
							},
							machines: [
						{
							id: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							agentName: "opsy",
							name: "rae-mbp",
							host: { hostname: "rae-mbp", os: "darwin", arch: "arm64", version: "0.4.2" },
							status: "active",
							enrolledAt: "2026-08-11T14:20:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
							load: { connected: true, capacity: 2, used: 1, free: 1, diskPressure: false },
						},
							],
							codebases: [
						{
							id: "00000000-0000-4000-8000-0000000000d1",
							runnerId: "00000000-0000-4000-8000-0000000000c1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							agentId: "00000000-0000-4000-8000-0000000000a1",
							name: "norn",
							rootPath: "/Users/rae/projects/norn",
							state: "active",
							repositories: [
								{
									name: "norn",
									relPath: ".",
									defaultBranch: "main",
									remote: { hash: "9f2c1a", host: "github.com", pathTail: "northwind/norn" },
								},
								{
									name: "runner",
									relPath: "runner",
									defaultBranch: "main",
									remote: { hash: "41ba07", host: "github.com", pathTail: "northwind/runner" },
								},
							],
							sharedFiles: ["AGENTS.md", "Makefile", "docker-compose.yml"],
							runtimes: ["process", "docker"],
							tools: [
								{ name: "claude", version: "2.0.1" },
								{ name: "codex", version: "0.9.4" },
							],
							previewGateway: "reachable",
							connectedAt: "2026-08-11T14:31:00Z",
							lastSeenAt: "2026-08-24T01:58:00Z",
						},
							],
							folders: "ready",
						},
					],
				},
				agents: [
					{
						agent: {
							id: "00000000-0000-4000-8000-0000000000a1",
							workspaceId: "00000000-0000-4000-8000-0000000000b1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							ownerAccountId: "00000000-0000-4000-8000-0000000000f1",
							name: "opsy",
							status: "active",
							actionLimit: 60,
							createdAt: "2026-07-02T09:00:00Z",
						},
						ownerName: "Rae Chen",
						ownerEmail: "rae@northwind.co",
					},
				],
				failure: { kind: "forbidden" },
			},
		}
	: {};
