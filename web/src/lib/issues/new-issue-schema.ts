import { z } from "zod";

import type { Document } from "$lib/editor/document";

export const newIssueSchema = z.object({
	teamId: z.string().trim().min(1, "Choose a team for this issue."),
	title: z
		.string()
		.trim()
		.min(1, "Give the issue a title.")
		.max(200, "Keep the title under 200 characters."),
	stateId: z.string().trim().default(""),
	priority: z.enum(["urgent", "high", "medium", "low", "none"]).default("none"),
	assigneeId: z.string().trim().default(""),
	projectId: z.string().trim().default(""),
	cycleId: z.string().trim().default(""),
	labelIds: z.array(z.string()).default([]),
	dueOn: z
		.string()
		.trim()
		.default("")
		.refine((raw) => raw === "" || /^\d{4}-\d{2}-\d{2}$/.test(raw), "Use a date like 2026-09-01."),
	createMore: z.boolean().default(false),
});

export type NewIssueInput = z.infer<typeof newIssueSchema>;

/**
 * What a refused creation hands back so the writing survives it. The description travels as the
 * document the editor held, because rendering it to text and reading it back would quietly drop
 * a mention, an attachment or a linked issue.
 */
export type NewIssuePrefill = Partial<NewIssueInput> & { description?: Document };
