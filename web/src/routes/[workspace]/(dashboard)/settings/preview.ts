import type { Team } from "$lib/team/teams";
import type { WorkspaceSettings } from "$lib/workspace/settings";
import type { StorageReading } from "$lib/workspace/storage";

export type WorkspaceSettingsPreview = {
	settings: WorkspaceSettings;
	teams: Team[];
	storage?: StorageReading;
};

export const workspaceSettingsPreviewStates: Record<string, WorkspaceSettingsPreview> = import.meta
	.env.DEV
	? {
			ready: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
			},
			default_team: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						defaultTeamId: "00000000-0000-4000-8000-000000000102",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
					{
						id: "00000000-0000-4000-8000-000000000102",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "PLT",
						name: "Data Platform",
						visibility: "private",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
				],
			},
			no_teams: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [],
			},
			saved: {
				settings: {
					kind: "saved",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind Trading",
						status: "active",
						timezone: "America/New_York",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
			},
			pending_deletion: {
				settings: {
					kind: "pending_deletion",
					purgeAfter: "2026-09-01T09:00:00Z",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "pending_deletion",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
						deletionRequestedAt: "2026-08-02T09:00:00Z",
						purgeAfter: "2026-09-01T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
			},
			unavailable: {
				settings: { kind: "unavailable" },
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
			},
			metered: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: {
					kind: "metered",
					storedBytes: 3_650_722_201,
					maxBytes: 10_737_418_240,
					remainingBytes: 7_086_696_039,
					percent: 34,
					pressure: "comfortable",
					measuredAt: "2026-08-12T08:41:00Z",
				},
			},
			nearly_full: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: {
					kind: "metered",
					storedBytes: 10_190_182_154,
					maxBytes: 10_737_418_240,
					remainingBytes: 547_236_086,
					percent: 95,
					pressure: "nearly_full",
					measuredAt: "2026-08-12T08:41:00Z",
				},
			},
			full: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: {
					kind: "metered",
					storedBytes: 10_737_418_240,
					maxBytes: 10_737_418_240,
					remainingBytes: 0,
					percent: 100,
					pressure: "full",
					measuredAt: "2026-08-12T08:41:00Z",
				},
			},
			unlimited: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: {
					kind: "unlimited",
					storedBytes: 3_650_722_201,
					measuredAt: "2026-08-12T08:41:00Z",
				},
			},
			never_measured: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: {
					kind: "metered",
					storedBytes: 0,
					maxBytes: 10_737_418_240,
					remainingBytes: 10_737_418_240,
					percent: 0,
					pressure: "comfortable",
				},
			},
			storage_unavailable: {
				settings: {
					kind: "ready",
					workspace: {
						id: "00000000-0000-4000-8000-000000000000",
						slug: "northwind",
						name: "Northwind",
						status: "active",
						timezone: "Europe/London",
						createdAt: "2026-01-04T09:00:00Z",
					},
				},
				teams: [
					{
						id: "00000000-0000-4000-8000-000000000101",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "MOB",
						name: "Mobile",
						visibility: "public",
						status: "active",
						createdAt: "2026-01-04T09:00:00Z",
					},
				],
				storage: { kind: "unavailable" },
			},
		}
	: {};
