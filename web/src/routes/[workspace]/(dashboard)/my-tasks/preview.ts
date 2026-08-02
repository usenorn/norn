import type { Task } from "$lib/tasks/types";

export type TaskBucket = { key: string; label: string; emphasis: boolean; tasks: Task[] };

export type MyTasksPreview = { buckets: TaskBucket[] };

export const myTasksPreviewStates: Record<string, MyTasksPreview> = import.meta.env.DEV
	? {
			default: {
				buckets: [
					{
						key: "overdue",
						label: "Overdue",
						emphasis: true,
						tasks: [
							{
								id: "BIL-112",
								title: "Proration is off by one day on annual plans",
								status: "review",
								priority: "urgent",
								assignee: "Rae Okafor",
								date: "Jul 28",
								labels: [{ name: "Bug", color: "magenta" }],
								project: "Billing",
								cycle: "24",
							},
							{
								id: "MOB-241",
								title: "Offline queue drops edits on reconnect",
								status: "started",
								priority: "urgent",
								assignee: "Rae Okafor",
								date: "Jul 29",
								labels: [{ name: "Bug", color: "magenta" }],
								project: "Mobile",
								cycle: "24",
							},
						],
					},
					{
						key: "today",
						label: "Today",
						emphasis: false,
						tasks: [
							{
								id: "MOB-236",
								title: "Keyboard navigation for the board view",
								status: "review",
								priority: "medium",
								assignee: "Rae Okafor",
								date: "Jul 30",
								labels: [{ name: "Design", color: "violet" }],
								project: "Mobile",
								cycle: "24",
							},
							{
								id: "BIL-118",
								title: "Invoice PDFs should include the tax ID",
								status: "todo",
								priority: "high",
								assignee: "Rae Okafor",
								date: "Jul 30",
								labels: [],
								project: "Billing",
								cycle: "24",
							},
						],
					},
					{
						key: "week",
						label: "This week",
						emphasis: false,
						tasks: [
							{
								id: "MOB-238",
								title: "Ship the weekly digest email",
								status: "started",
								priority: "high",
								assignee: "Rae Okafor",
								date: "Aug 1",
								labels: [{ name: "Needs spec", color: "cyan" }],
								project: "Mobile",
								cycle: "24",
							},
							{
								id: "GRW-64",
								title: "Rewrite the empty states across settings",
								status: "todo",
								priority: "medium",
								assignee: "Rae Okafor",
								date: "Aug 2",
								labels: [{ name: "Design", color: "violet" }],
								project: "Growth",
								cycle: "24",
							},
							{
								id: "MOB-244",
								title: "Cache avatars between sessions",
								status: "todo",
								priority: "low",
								assignee: "Rae Okafor",
								date: "Aug 3",
								labels: [{ name: "Infra", color: "blue" }],
								project: "Mobile",
								cycle: "24",
							},
						],
					},
					{
						key: "later",
						label: "Later",
						emphasis: false,
						tasks: [
							{
								id: "GRW-71",
								title: "Instrument the onboarding funnel",
								status: "todo",
								priority: "medium",
								assignee: "Rae Okafor",
								date: "Aug 8",
								labels: [{ name: "Infra", color: "blue" }],
								project: "Growth",
								cycle: "24",
							},
							{
								id: "BIL-121",
								title: "Dunning emails for failed cards",
								status: "backlog",
								priority: "high",
								assignee: "Rae Okafor",
								date: null,
								labels: [{ name: "Needs spec", color: "cyan" }],
								project: "Billing",
								cycle: null,
							},
							{
								id: "MOB-249",
								title: "Drop support for iOS 15",
								status: "backlog",
								priority: "low",
								assignee: "Rae Okafor",
								date: null,
								labels: [],
								project: "Mobile",
								cycle: null,
							},
						],
					},
				],
			},
			empty: { buckets: [] },
		}
	: {};
