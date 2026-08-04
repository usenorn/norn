import type { Team } from "$lib/team/teams";
import type { ViewFailure, ViewListing } from "$lib/views/views";

export type ViewsPreview = {
	listing: ViewListing;
	teams?: Team[];
	failure?: ViewFailure;
	removing?: string;
	editing?: string;
	working?: string;
};

export const viewsPreviewStates: Record<string, ViewsPreview> = import.meta.env.DEV
	? {
			loading: { listing: { kind: "loading" } },
			unavailable: { listing: { kind: "unavailable" } },
			empty: { listing: { kind: "empty" } },
			ready: {
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
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Urgent and unassigned",
							sharing: "personal",
							filter: {
								all: [
									{ field: "priority", op: "is", values: ["urgent"] },
									{ field: "assignee", op: "is_not_set" },
								],
							},
							sort: [],
							editable: true,
							createdByAccountId: "00000000-0000-4000-8000-000000000201",
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-01T09:00:00Z",
							updatedAt: "2026-08-01T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000902",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Mobile in flight",
							sharing: "team",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamName: "Mobile",
							filter: { field: "stateCategory", op: "is", values: ["active"] },
							sort: [{ field: "dueOn" }],
							groupBy: "assignee",
							editable: true,
							createdByAccountId: "00000000-0000-4000-8000-000000000201",
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-02T09:00:00Z",
							updatedAt: "2026-08-02T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000903",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Everything overdue",
							sharing: "workspace",
							filter: { field: "dueOn", op: "before", values: ["2026-08-01"] },
							sort: [],
							editable: false,
							createdByAccountId: "00000000-0000-4000-8000-000000000202",
							createdByName: "Grace Hopper",
							createdAt: "2026-07-20T09:00:00Z",
							updatedAt: "2026-07-20T09:00:00Z",
						},
					],
				},
			},
			single: {
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Urgent and unassigned",
							sharing: "personal",
							filter: { field: "priority", op: "is", values: ["urgent"] },
							sort: [],
							editable: true,
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-01T09:00:00Z",
							updatedAt: "2026-08-01T09:00:00Z",
						},
					],
				},
			},
			removing_shared: {
				removing: "00000000-0000-4000-8000-000000000902",
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000902",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Mobile in flight",
							sharing: "team",
							teamId: "00000000-0000-4000-8000-000000000101",
							teamName: "Mobile",
							filter: { field: "stateCategory", op: "is", values: ["active"] },
							sort: [],
							editable: true,
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-02T09:00:00Z",
							updatedAt: "2026-08-02T09:00:00Z",
						},
					],
				},
			},
			sharing_changed: {
				removing: "00000000-0000-4000-8000-000000000901",
				failure: { kind: "sharing_changed", sharing: "workspace", team: "" },
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Urgent and unassigned",
							sharing: "personal",
							filter: { field: "priority", op: "is", values: ["urgent"] },
							sort: [],
							editable: true,
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-01T09:00:00Z",
							updatedAt: "2026-08-01T09:00:00Z",
						},
					],
				},
			},
			forbidden: {
				failure: { kind: "forbidden" },
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000903",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Everything overdue",
							sharing: "workspace",
							filter: { field: "dueOn", op: "before", values: ["2026-08-01"] },
							sort: [],
							editable: false,
							createdByName: "Grace Hopper",
							createdAt: "2026-07-20T09:00:00Z",
							updatedAt: "2026-07-20T09:00:00Z",
						},
					],
				},
			},
			editing: {
				editing: "00000000-0000-4000-8000-000000000901",
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
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Urgent and unassigned",
							sharing: "personal",
							filter: { field: "priority", op: "is", values: ["urgent"] },
							sort: [],
							editable: true,
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-01T09:00:00Z",
							updatedAt: "2026-08-01T09:00:00Z",
						},
					],
				},
			},
			reordering: {
				working: "00000000-0000-4000-8000-000000000901",
				listing: {
					kind: "ready",
					views: [
						{
							id: "00000000-0000-4000-8000-000000000901",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Urgent and unassigned",
							sharing: "personal",
							filter: { field: "priority", op: "is", values: ["urgent"] },
							sort: [],
							editable: true,
							createdByName: "Ada Lovelace",
							createdAt: "2026-08-01T09:00:00Z",
							updatedAt: "2026-08-01T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000903",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Everything overdue",
							sharing: "workspace",
							filter: { field: "dueOn", op: "before", values: ["2026-08-01"] },
							sort: [],
							editable: false,
							createdByName: "Grace Hopper",
							createdAt: "2026-07-20T09:00:00Z",
							updatedAt: "2026-07-20T09:00:00Z",
						},
					],
				},
			},
		}
	: {};
