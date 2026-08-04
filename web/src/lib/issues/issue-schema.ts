import { z } from "zod";

export const issueEditSchema = z.object({
	title: z
		.string()
		.trim()
		.min(1, "Give the issue a title.")
		.max(200, "Keep the title under 200 characters."),
	description: z.string().max(20000, "Keep the description under 20,000 characters.").default(""),
	priority: z.enum(["urgent", "high", "medium", "low", "none"]).default("none"),
	assigneeId: z.string().trim().default(""),
	estimate: z
		.string()
		.trim()
		.default("")
		.refine((raw) => raw === "" || /^\d+$/.test(raw), "Use a whole number of points.")
		.refine((raw) => raw === "" || Number(raw) >= 1, "An estimate is at least one point.")
		.refine((raw) => raw === "" || Number(raw) <= 1000, "Keep the estimate under 1,000 points."),
	dueOn: z
		.string()
		.trim()
		.default("")
		.refine((raw) => raw === "" || /^\d{4}-\d{2}-\d{2}$/.test(raw), "Use a date like 2026-09-01."),
});

export type IssueEditInput = z.infer<typeof issueEditSchema>;
