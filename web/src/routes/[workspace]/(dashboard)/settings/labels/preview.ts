import type { LabelBoard, LabelUsage } from "$lib/labels/labels";
import type { Team } from "$lib/team/teams";

export type LabelsPreview = {
	board: LabelBoard;
	usage?: LabelUsage;
	teams?: Team[];
	opens?: "editor" | "merge" | "delete";
};

export const labelsPreviewStates: Record<string, LabelsPreview> = import.meta.env.DEV
	? {
			loading: { board: { kind: "loading" } },
			unavailable: { board: { kind: "unavailable" } },
			empty: { board: { kind: "ready", labels: [], groups: [] } },
			list: {
				usage: { kind: "counted", counts: {
					"00000000-0000-4000-8000-000000000701": 86,
					"00000000-0000-4000-8000-000000000702": 41,
					"00000000-0000-4000-8000-000000000703": 52,
					"00000000-0000-4000-8000-000000000704": 34,
					"00000000-0000-4000-8000-000000000705": 12,
					"00000000-0000-4000-8000-000000000706": 29,
				} },
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
					groups: [],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000701",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Bug",
							description: "Something shipped is wrong",
							color: "magenta",
						},
						{
							id: "00000000-0000-4000-8000-000000000702",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Chore",
							description: "Necessary, not user-visible",
							color: "neutral",
						},
						{
							id: "00000000-0000-4000-8000-000000000703",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Design",
							description: "",
							color: "violet",
						},
						{
							id: "00000000-0000-4000-8000-000000000704",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Needs spec",
							description: "Blocked until product answers",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000705",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Needs review",
							description: "",
							color: "blue",
						},
						{
							id: "00000000-0000-4000-8000-000000000706",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							teamId: "00000000-0000-4000-8000-000000000101",
							name: "Infra",
							description: "Build, deploy and sync plumbing",
							color: "cyan",
						},
					],
				},
			},
			uncounted: {
				usage: { kind: "uncounted" },
				board: {
					kind: "ready",
					groups: [],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000751",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Bug",
							description: "Something shipped is wrong",
							color: "magenta",
						},
						{
							id: "00000000-0000-4000-8000-000000000752",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Chore",
							description: "Necessary, not user-visible",
							color: "neutral",
						},
					],
				},
			},
			groups: {
				usage: { kind: "counted", counts: {
					"00000000-0000-4000-8000-000000000711": 86,
					"00000000-0000-4000-8000-000000000712": 41,
					"00000000-0000-4000-8000-000000000713": 34,
					"00000000-0000-4000-8000-000000000714": 18,
				} },
				board: {
					kind: "ready",
					groups: [
						{
							id: "00000000-0000-4000-8000-000000000601",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Type",
						},
						{
							id: "00000000-0000-4000-8000-000000000602",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Stage",
						},
					],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000711",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000601",
							name: "Bug",
							description: "Something shipped is wrong",
							color: "magenta",
						},
						{
							id: "00000000-0000-4000-8000-000000000712",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000601",
							name: "Chore",
							description: "Necessary, not user-visible",
							color: "neutral",
						},
						{
							id: "00000000-0000-4000-8000-000000000713",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							groupId: "00000000-0000-4000-8000-000000000602",
							name: "Needs spec",
							description: "Blocked until product answers",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000714",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Spec",
							description: "Older duplicate of Needs spec",
							color: "orchid",
						},
					],
				},
			},
			edit: {
				opens: "editor",
				usage: { kind: "counted", counts: { "00000000-0000-4000-8000-000000000721": 29 } },
				board: {
					kind: "ready",
					groups: [
						{
							id: "00000000-0000-4000-8000-000000000603",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Type",
						},
					],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000721",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Tech debt",
							description: "Paying for a shortcut taken earlier",
							color: "cyan",
						},
					],
				},
			},
			merge: {
				opens: "merge",
				usage: { kind: "counted", counts: {
					"00000000-0000-4000-8000-000000000731": 18,
					"00000000-0000-4000-8000-000000000732": 34,
				} },
				board: {
					kind: "ready",
					groups: [],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000731",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Spec",
							description: "Older duplicate of Needs spec",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000732",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Needs spec",
							description: "Blocked until product answers",
							color: "orchid",
						},
					],
				},
			},
			delete: {
				opens: "delete",
				usage: { kind: "counted", counts: {
					"00000000-0000-4000-8000-000000000741": 34,
					"00000000-0000-4000-8000-000000000742": 18,
				} },
				board: {
					kind: "ready",
					groups: [],
					labels: [
						{
							id: "00000000-0000-4000-8000-000000000741",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Needs spec",
							description: "Blocked until product answers",
							color: "orchid",
						},
						{
							id: "00000000-0000-4000-8000-000000000742",
							workspaceId: "00000000-0000-4000-8000-000000000000",
							name: "Spec",
							description: "Older duplicate of Needs spec",
							color: "orchid",
						},
					],
				},
			},
		}
	: {};
