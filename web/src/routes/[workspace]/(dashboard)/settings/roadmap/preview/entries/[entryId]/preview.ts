import type { RoadmapPreviewEntryView } from "$lib/roadmap/admin";

export const roadmapPreviewEntryStates: Record<string, RoadmapPreviewEntryView> = import.meta.env.DEV
	? {
			loading: { kind: "loading" },
			forbidden: { kind: "forbidden" },
			not_found: { kind: "not_found" },
			unavailable: { kind: "unavailable" },
			entry: {
				kind: "ready",
				roadmapTitle: "Northwind field roadmap",
				item: {
					id: "fq7dz2",
					kind: "project",
					title: "Crews keep working offline",
					description:
						"Jobs, photos and signatures save on the device and sync once a crew is back in range.\nWe start with the job sheet and add the rest one screen at a time.",
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
			},
			entry_issue: {
				kind: "ready",
				roadmapTitle: "Northwind field roadmap",
				item: {
					id: "lx2yq9",
					kind: "issue",
					title: "Larger tap targets on the job sheet",
					description: "Easier to use with gloves on.",
					status: "completed",
					timeline: { endOn: "2026-09-30" },
				},
			},
		}
	: {};
