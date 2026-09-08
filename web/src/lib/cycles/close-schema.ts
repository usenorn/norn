import { z } from "zod";

export const closeCycleSchema = z
	.object({
		reviewedIssueIds: z.array(z.string()).default([]),
		decisions: z.record(z.string(), z.enum(["next", "backlog", "keep"])).default({}),
	})
	.refine((entered) => entered.reviewedIssueIds.every((issueId) => entered.decisions[issueId]), {
		message: "Decide where every unfinished issue goes.",
		path: ["decisions"],
	});

export type CloseCycleInput = z.infer<typeof closeCycleSchema>;
