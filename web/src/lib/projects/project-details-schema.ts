import { z } from "zod";
import { agentInstructionsMaxLength, agentInstructionsTooLong } from "$lib/agents/instructions";

export const projectDetailsSchema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Give the project a name.")
		.max(80, "Keep the name under 80 characters."),
	description: z.string().trim().max(4000, "Keep the description under 4000 characters."),
	agentInstructions: z
		.string()
		.trim()
		.max(agentInstructionsMaxLength, agentInstructionsTooLong),
	targetOn: z
		.string()
		.trim()
		.refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value), {
			message: "Use a date like 2026-09-30.",
		}),
	leadAccountId: z.string().trim(),
	teamIds: z.array(z.string()),
});

export type ProjectDetailsInput = z.infer<typeof projectDetailsSchema>;
