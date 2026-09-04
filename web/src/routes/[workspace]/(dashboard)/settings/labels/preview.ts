import type { LabelBoard } from "$lib/labels/labels";
import type { Team } from "$lib/team/teams";

export type LabelsPreview = {
	board: LabelBoard;
	teams?: Team[];
};

export const labelsPreviewStates: Record<string, LabelsPreview> = import.meta.env.DEV
	? {
			loading: { board: { kind: "loading" } },
			unavailable: { board: { kind: "unavailable" } },
			empty: { board: { kind: "ready", labels: [], groups: [] } },
			ready: {
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
				board: {
					kind: "ready",
					groups: [
						{
							id: "00000000-0000-4000-8000-000000000601",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Severity",
						},
					],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000701",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000601",
							name: "Blocker",
							color: "magenta",
						},
						{
							id: "00000000-0000-4000-8000-000000000702",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000601",
							name: "Major",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000703",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000601",
							name: "Minor",
							color: "violet",
						},
						{
							id: "00000000-0000-4000-8000-000000000704",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Needs spec",
							color: "cyan",
						},
						{
							id: "00000000-0000-4000-8000-000000000705",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Regression",
							color: "blue",
						},
						{
							id: "00000000-0000-4000-8000-000000000706",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							teamId: "00000000-0000-4000-8000-000000000101",
							name: "Crash",
							color: "neutral",
						},
					],
				},
			},
			duplicates: {
				board: {
					kind: "ready",
					groups: [],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000801",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Bug",
							color: "magenta",
						},
						{
							id: "00000000-0000-4000-8000-000000000802",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Defect",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000803",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "bug-report",
							color: "violet",
						},
					],
				},
			},
		}
	: {};
