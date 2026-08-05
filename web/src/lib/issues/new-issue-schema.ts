import { z } from "zod";

export const newIssueSchema = z.object({
	teamId: z.string().trim().min(1, "Choose a team for this issue."),
	title: z
		.string()
		.trim()
		.min(1, "Give the issue a title.")
		.max(200, "Keep the title under 200 characters."),
	description: z.string().max(20000, "Keep the description under 20,000 characters.").default(""),
	stateId: z.string().trim().default(""),
	priority: z.enum(["urgent", "high", "medium", "low", "none"]).default("none"),
	assigneeId: z.string().trim().default(""),
	projectId: z.string().trim().default(""),
	labelIds: z.array(z.string()).default([]),
	dueOn: z
		.string()
		.trim()
		.default("")
		.refine((raw) => raw === "" || /^\d{4}-\d{2}-\d{2}$/.test(raw), "Use a date like 2026-09-01."),
	createMore: z.boolean().default(false),
});

export type NewIssueInput = z.infer<typeof newIssueSchema>;
