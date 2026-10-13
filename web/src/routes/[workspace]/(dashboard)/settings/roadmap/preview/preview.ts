import type { RoadmapPreviewView } from "$lib/roadmap/admin";

export const roadmapPreviewStates: Record<string, RoadmapPreviewView> = import.meta.env.DEV
	? {
			loading: { kind: "loading" },
			forbidden: { kind: "forbidden" },
			unavailable: { kind: "unavailable" },
			empty: {
				kind: "ready",
				preview: {
					version: 1,
					digest: "4be0c2d17a9e3f58",
					roadmap: {
						title: "Northwind field roadmap",
						description: "What the field app team is building next.",
						publishedAt: "0001-01-01T00:00:00Z",
						projects: [],
						issues: [],
					},
					blockers: [],
				},
				entries: [],
			},
			ready: {
				kind: "ready",
				preview: {
					version: 7,
					digest: "9d3a61f0c2b8e475",
					roadmap: {
						title: "Northwind field roadmap",
						description:
							"What the field app team is building next.\nDates move as we learn; this page always shows the plan we would publish.",
						publishedAt: "0001-01-01T00:00:00Z",
						projects: [
							{
								id: "fq7dz2",
								kind: "project",
								title: "Crews keep working offline",
								description: "Jobs, photos and signatures save on the device and sync once a crew is back in range.",
								status: "in_progress",
								timeline: { startOn: "2026-09-01", endOn: "2026-12-15" },
								progress: 40,
								issues: [
									{
										id: "hm3kv8",
										kind: "issue",
										title: "Merge edits two crews make to one job",
										status: "in_progress",
										timeline: { endOn: "2026-11-20" },
									},
									{ id: "pw6tn1", kind: "issue", title: "Queue photo uploads", status: "planned" },
								],
							},
							{
								id: "zc8rb4",
								kind: "project",
								title: "Route planning for dispatchers",
								status: "planned",
								timeline: { startOn: "2027-01-10", endOn: "2027-03-31" },
							},
						],
						issues: [
							{
								id: "lx2yq9",
								kind: "issue",
								title: "Larger tap targets on the job sheet",
								description: "Easier to use with gloves on.",
								status: "completed",
								timeline: { endOn: "2026-09-30" },
							},
						],
					},
					blockers: [],
				},
				entries: [],
			},
			blocked: {
				kind: "ready",
				preview: {
					version: 9,
					digest: "e17c5ab93f0d6284",
					roadmap: {
						title: "",
						publishedAt: "0001-01-01T00:00:00Z",
						projects: [
							{
								id: "fq7dz2",
								kind: "project",
								title: "Crews keep working offline",
								status: "in_progress",
								timeline: { startOn: "2026-09-01", endOn: "2026-12-15" },
							},
						],
						issues: [],
					},
					blockers: [
						{ code: "roadmap_title_required" },
						{ code: "entry_unreachable", entryId: "00000000-0000-4000-8000-000000002002" },
					],
				},
				entries: [
					{
						id: "00000000-0000-4000-8000-000000002001",
						publicId: "fq7dz2",
						kind: "project",
						projectId: "00000000-0000-4000-8000-000000002101",
						publicTitle: "Crews keep working offline",
						publicDescription: "",
						status: "in_progress",
						timeline: { startOn: "2026-09-01", endOn: "2026-12-15" },
						progressMode: "hidden",
						progressPercent: 0,
						visibility: {
							issueLists: true,
							projectDescriptions: false,
							issueDescriptions: false,
							status: true,
							timeline: true,
							progress: true,
						},
						position: 0,
						reachable: true,
						published: false,
						withheld: false,
						source: { label: "Field app offline mode" },
						progressCoverage: 0,
					},
					{
						id: "00000000-0000-4000-8000-000000002002",
						publicId: "vb5ks3",
						kind: "project",
						projectId: "00000000-0000-4000-8000-000000002102",
						publicTitle: "Customer self-service portal",
						publicDescription: "",
						status: "planned",
						timeline: {},
						progressMode: "hidden",
						progressPercent: 0,
						visibility: {
							issueLists: true,
							projectDescriptions: false,
							issueDescriptions: false,
							status: true,
							timeline: true,
							progress: true,
						},
						position: 1,
						reachable: false,
						published: false,
						withheld: false,
						progressCoverage: 0,
					},
				],
			},
		}
	: {};
