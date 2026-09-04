import type { CreateTeamInput } from "$lib/team/create-team-schema";
import type { TeamCreationFailure, TeamListing, TeamListView } from "$lib/team/teams";

export type TeamsPreview = {
	listing: TeamListing;
	view?: TeamListView;
	form?: Partial<CreateTeamInput>;
	failure?: TeamCreationFailure;
	busy?: boolean;
};

export const teamsPreviewStates: Record<string, TeamsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			empty: { listing: { kind: "empty" }, view: "create" },
			ready: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000102",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "PLT",
							name: "Data Platform",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "private",
							status: "active",
							createdAt: "2026-02-11T09:00:00Z",
						},
					],
				},
			},
			archived: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000103",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "LEG",
							name: "Legacy Billing",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "archived",
							createdAt: "2025-06-02T09:00:00Z",
							archivedAt: "2026-03-18T09:00:00Z",
						},
					],
				},
			},
			archived_empty: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
			},
			create: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
				view: "create",
			},
			create_invalid: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
				view: "create",
				form: { name: "D", key: "m0b" },
			},
			create_key_taken: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
				view: "create",
				form: { name: "Mobile", key: "MOB" },
				failure: { kind: "key_taken", key: "MOB", suggestions: ["MOBI", "MOBIL", "MBL"] },
			},
			creating: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
				view: "create",
				form: { name: "Data Platform", key: "PLT" },
				busy: true,
			},
			created: {
				listing: {
					kind: "created",
					team: {
						id: "00000000-0000-4000-8000-000000000102",
						workspaceId: "00000000-0000-4000-8000-000000000000",
						key: "PLT",
						name: "Data Platform",
						description: "",
						icon: "",
						iconColor: "neutral",
						estimation: "none",
						visibility: "private",
						status: "active",
						createdAt: "2026-02-11T09:00:00Z",
					},
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000102",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "PLT",
							name: "Data Platform",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "private",
							status: "active",
							createdAt: "2026-02-11T09:00:00Z",
						},
					],
				},
			},
			forbidden: {
				listing: {
					kind: "ready",
					teams: [
						{
							id: "00000000-0000-4000-8000-000000000101",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							key: "MOB",
							name: "Mobile",
							description: "",
							icon: "",
							iconColor: "neutral",
							estimation: "none",
							visibility: "public",
							status: "active",
							createdAt: "2026-01-04T09:00:00Z",
						},
					],
				},
				view: "create",
				failure: { kind: "forbidden" },
			},
			unavailable: { listing: { kind: "unavailable" } },
		}
	: {};
