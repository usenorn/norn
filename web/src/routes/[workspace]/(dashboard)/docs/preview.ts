import type { DocTree } from "$lib/docs/docs";

export type DocsPreview = { tree: DocTree };

export const docsPreviewStates: Record<string, DocsPreview> = import.meta.env.DEV
	? {
			loading: { tree: { kind: "loading" } },
			unavailable: { tree: { kind: "unavailable" } },
			empty: { tree: { kind: "empty" } },
			ready: {
				tree: {
					kind: "ready",
					partial: false,
					docs: [
						{
							id: "00000000-0000-4000-8000-000000000a01",
							title: "On-call runbook",
							audience: { visibility: "workspace" },
							status: "active",
							version: 4,
							createdAt: "2026-09-01T09:00:00Z",
							updatedAt: "2026-10-20T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000a02",
							parentId: "00000000-0000-4000-8000-000000000a01",
							title: "Restarting the payment worker",
							audience: { visibility: "workspace" },
							status: "active",
							version: 2,
							createdAt: "2026-09-02T09:00:00Z",
							updatedAt: "2026-10-21T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000a03",
							title: "Checkout rebuild decisions",
							audience: { visibility: "project", projectId: "00000000-0000-4000-8000-000000000801" },
							status: "active",
							version: 1,
							createdAt: "2026-09-03T09:00:00Z",
							updatedAt: "2026-10-22T09:00:00Z",
						},
						{
							id: "00000000-0000-4000-8000-000000000a04",
							title: "Interview notes",
							audience: { visibility: "personal" },
							status: "active",
							version: 1,
							createdAt: "2026-09-04T09:00:00Z",
							updatedAt: "2026-10-23T09:00:00Z",
						},
					],
					sections: [
						{
							key: "workspace",
							title: "Workspace",
							visibility: "workspace",
							nodes: [
								{
									doc: {
										id: "00000000-0000-4000-8000-000000000a01",
										title: "On-call runbook",
										audience: { visibility: "workspace" },
										status: "active",
										version: 4,
										createdAt: "2026-09-01T09:00:00Z",
										updatedAt: "2026-10-20T09:00:00Z",
									},
									children: [
										{
											doc: {
												id: "00000000-0000-4000-8000-000000000a02",
												parentId: "00000000-0000-4000-8000-000000000a01",
												title: "Restarting the payment worker",
												audience: { visibility: "workspace" },
												status: "active",
												version: 2,
												createdAt: "2026-09-02T09:00:00Z",
												updatedAt: "2026-10-21T09:00:00Z",
											},
											children: [],
										},
									],
								},
							],
						},
						{
							key: "project:00000000-0000-4000-8000-000000000801",
							title: "Checkout rebuild",
							visibility: "project",
							projectId: "00000000-0000-4000-8000-000000000801",
							nodes: [
								{
									doc: {
										id: "00000000-0000-4000-8000-000000000a03",
										title: "Checkout rebuild decisions",
										audience: { visibility: "project", projectId: "00000000-0000-4000-8000-000000000801" },
										status: "active",
										version: 1,
										createdAt: "2026-09-03T09:00:00Z",
										updatedAt: "2026-10-22T09:00:00Z",
									},
									children: [],
								},
							],
						},
						{
							key: "personal",
							title: "Personal",
							visibility: "personal",
							nodes: [
								{
									doc: {
										id: "00000000-0000-4000-8000-000000000a04",
										title: "Interview notes",
										audience: { visibility: "personal" },
										status: "active",
										version: 1,
										createdAt: "2026-09-04T09:00:00Z",
										updatedAt: "2026-10-23T09:00:00Z",
									},
									children: [],
								},
							],
						},
					],
				},
			},
		}
	: {};
