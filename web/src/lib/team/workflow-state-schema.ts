import { z } from "zod";

export const workflowStateSchema = z.object({
	name: z
		.string()
		.trim()
		.min(2, "Enter a state name.")
		.max(40, "Keep the name under 40 characters."),
	category: z.enum(["not_started", "active", "complete", "abandoned"]).default("not_started"),
});

export type WorkflowStateInput = z.infer<typeof workflowStateSchema>;
