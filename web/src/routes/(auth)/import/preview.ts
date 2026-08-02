import type { ImportSession } from "$lib/import/types";
import type { ConnectInput } from "$lib/import/connect-schema";

export type ImportPreview = {
	session?: Partial<ImportSession>;
	form?: Partial<ConnectInput>;
};

export const importPreviewStates: Record<string, ImportPreview> = import.meta.env.DEV
	? {
			source: { session: { source: "jira", stage: { kind: "choose_source" } } },
			connect: {
				form: {
					site: "https://northwind.atlassian.net",
					email: "rae@northwind.co",
					token: "ATATT3xFfGF0T9k2",
				},
				session: { source: "jira", stage: { kind: "connect", failure: null } },
			},
			connfail: {
				form: {
					site: "https://northwind.atlassian.net",
					email: "rae@northwind.co",
					token: "ATATT3xFfGF0T9k2",
				},
				session: {
					source: "jira",
					stage: {
						kind: "connect",
						failure: {
							kind: "token_rejected",
							diagnostics: [
								{ key: "status", value: "401 Unauthorized" },
								{ key: "endpoint", value: "GET /rest/api/3/search" },
								{ key: "account", value: "rae@northwind.co" },
								{ key: "scopes", value: "read:jira-work missing" },
								{ key: "time", value: "14:19:52 UTC" },
							],
						},
					},
				},
			},
			map: {
				session: {
					source: "jira",
					stage: {
						kind: "map_fields",
						unresolvedOnly: false,
						mapping: {
							fieldsFound: 21,
							fieldsMapped: 18,
							affectedIssues: 128,
							groups: [
								{
									name: "Statuses",
									rows: [
										{
											source: "Backlog",
											volume: "312",
											value: "Backlog",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Selected for Dev",
											volume: "84",
											value: "Todo",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "In Progress",
											volume: "96",
											value: "In progress",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "In Review",
											volume: "41",
											value: "In review",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Done",
											volume: "702",
											value: "Done",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Won't Do",
											volume: "49",
											value: "Choose…",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: true,
										},
									],
								},
								{
									name: "Priorities",
									rows: [
										{
											source: "Highest",
											volume: "22",
											value: "Urgent",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "High",
											volume: "148",
											value: "High",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Medium",
											volume: "604",
											value: "Medium",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Low",
											volume: "402",
											value: "Low",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: false,
										},
										{
											source: "Lowest",
											volume: "108",
											value: "Choose…",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: true,
										},
									],
								},
								{
									name: "Labels",
									rows: [
										{
											source: "bug",
											volume: "214",
											value: "Bug",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: false,
										},
										{
											source: "design",
											volume: "88",
											value: "Design",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: false,
										},
										{
											source: "infra",
											volume: "62",
											value: "Infra",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: false,
										},
										{
											source: "needs-spec",
											volume: "44",
											value: "Needs spec",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: false,
										},
										{
											source: "tech-debt",
											volume: "31",
											value: "Choose…",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: true,
										},
									],
								},
							],
						},
					},
				},
			},
			unmapped: {
				session: {
					source: "jira",
					stage: {
						kind: "map_fields",
						unresolvedOnly: true,
						mapping: {
							fieldsFound: 21,
							fieldsMapped: 18,
							affectedIssues: 128,
							groups: [
								{
									name: "Statuses",
									rows: [
										{
											source: "Won't Do",
											volume: "49",
											value: "Choose…",
											options: [
												"Backlog",
												"Todo",
												"In progress",
												"In review",
												"Done",
												"Canceled",
												"Skip these issues",
											],
											needsDecision: true,
										},
									],
								},
								{
									name: "Priorities",
									rows: [
										{
											source: "Lowest",
											volume: "108",
											value: "Choose…",
											options: [
												"Urgent",
												"High",
												"Medium",
												"Low",
												"No priority",
												"Skip these issues",
											],
											needsDecision: true,
										},
									],
								},
								{
									name: "Labels",
									rows: [
										{
											source: "tech-debt",
											volume: "31",
											value: "Choose…",
											options: [
												"Bug",
												"Design",
												"Infra",
												"Needs spec",
												"Create label",
												"Don't import",
											],
											needsDecision: true,
										},
									],
								},
							],
						},
					},
				},
			},
			users: {
				session: {
					source: "jira",
					stage: {
						kind: "map_people",
						matching: {
							matched: 3,
							people: [
								{
									source: "Rae Okafor",
									email: "rae@northwind.co",
									volume: "142",
									value: "Rae Okafor",
									options: [
										"Rae Okafor",
										"Jun Park",
										"Milo Vance",
										"Ada Ling",
										"Invite by email",
										"Leave unassigned",
									],
									needsDecision: false,
								},
								{
									source: "Jun Park",
									email: "jun@northwind.co",
									volume: "96",
									value: "Jun Park",
									options: [
										"Rae Okafor",
										"Jun Park",
										"Milo Vance",
										"Ada Ling",
										"Invite by email",
										"Leave unassigned",
									],
									needsDecision: false,
								},
								{
									source: "M. Vance",
									email: "mvance@contractor.io",
									volume: "48",
									value: "Choose…",
									options: [
										"Rae Okafor",
										"Jun Park",
										"Milo Vance",
										"Ada Ling",
										"Invite by email",
										"Leave unassigned",
									],
									needsDecision: true,
								},
								{
									source: "Deactivated user",
									email: "no email in export",
									volume: "31",
									value: "Leave unassigned",
									options: [
										"Rae Okafor",
										"Jun Park",
										"Milo Vance",
										"Ada Ling",
										"Invite by email",
										"Leave unassigned",
									],
									needsDecision: false,
								},
							],
						},
					},
				},
			},
			preview: {
				session: {
					source: "jira",
					stage: {
						kind: "preview",
						plan: {
							counts: [
								{ value: "1,284", label: "Issues", tone: "normal" },
								{ value: "3,908", label: "Comments", tone: "normal" },
								{ value: "18", label: "Labels", tone: "normal" },
								{ value: "3", label: "Projects", tone: "normal" },
							],
							destinations: [
								{ label: "Mobile · MOB", count: "612" },
								{ label: "Billing · BIL", count: "402" },
								{ label: "Growth · GRW", count: "270" },
							],
							excluded: [
								"Attachments over 25 MB — 14 files",
								"Jira sprints. Norn cycles are made from dates instead.",
								"Automation rules and workflow conditions",
								'Issue links of type "relates to", kept as a comment',
							],
						},
					},
				},
			},
			running: {
				session: {
					source: "jira",
					stage: {
						kind: "running",
						detached: false,
						notifyEmail: "rae@northwind.co",
						progress: {
							percent: 48,
							phase: "Creating issues · 612 of 1,284",
							meta: "2m 14s elapsed · about 2m left",
						},
						log: [
							{
								at: "14:22:01",
								message: "Connected to northwind.atlassian.net",
								tone: "normal",
							},
							{ at: "14:22:04", message: "Read 1,284 issues and 3,908 comments", tone: "normal" },
							{ at: "14:22:19", message: "Created project MOB · 612 issues", tone: "normal" },
							{ at: "14:23:40", message: "Created project BIL · 402 issues", tone: "normal" },
							{ at: "14:24:15", message: "Skipped 3 attachments over 25 MB", tone: "warning" },
							{ at: "14:24:16", message: "Creating issues 613 to 1,284", tone: "active" },
						],
					},
				},
			},
			background: {
				session: {
					source: "jira",
					stage: {
						kind: "running",
						detached: true,
						notifyEmail: "rae@northwind.co",
						progress: {
							percent: 48,
							phase: "Creating issues · 612 of 1,284",
							meta: "2m 14s elapsed · about 2m left",
						},
						log: [],
					},
				},
			},
			done: {
				session: {
					source: "jira",
					stage: {
						kind: "finished",
						outcome: {
							kind: "complete",
							imported: "1,284",
							landedIn: "Mobile, Billing and Growth",
							primaryTeam: "Mobile",
							finishedAt: "14:26",
							duration: "4m 12s",
							counts: [
								{ value: "1,284", label: "Issues", tone: "normal" },
								{ value: "3,908", label: "Comments", tone: "normal" },
								{ value: "18", label: "Labels", tone: "normal" },
								{ value: "0", label: "Skipped", tone: "muted" },
							],
						},
					},
				},
			},
			skipped: {
				session: {
					source: "jira",
					stage: {
						kind: "finished",
						outcome: {
							kind: "with_skips",
							imported: "1,247",
							total: "1,284",
							primaryTeam: "Mobile",
							skippedTotal: "37",
							finishedAt: "14:26",
							duration: "4m 30s",
							counts: [
								{ value: "1,247", label: "Issues", tone: "normal" },
								{ value: "3,908", label: "Comments", tone: "normal" },
								{ value: "18", label: "Labels", tone: "normal" },
								{ value: "37", label: "Skipped", tone: "warning" },
							],
							skipped: [
								{ kind: "attachment", reason: "Larger than 25 MB", count: "14" },
								{
									kind: "assignee",
									reason: "No matching person — imported unassigned",
									count: "19",
								},
								{
									kind: "description",
									reason: "Used a Jira macro with no equivalent",
									count: "4",
								},
							],
						},
					},
				},
			},
			failed: {
				session: {
					source: "jira",
					stage: {
						kind: "finished",
						outcome: {
							kind: "failed",
							stoppedAfter: "612",
							resumeAt: "613",
							total: "1,284",
							stoppedAt: "14:24",
							counts: [
								{ value: "612", label: "Issues in", tone: "normal" },
								{ value: "1,204", label: "Comments", tone: "normal" },
								{ value: "12", label: "Labels", tone: "normal" },
								{ value: "672", label: "Not imported", tone: "danger" },
							],
						},
					},
				},
			},
		}
	: {};
