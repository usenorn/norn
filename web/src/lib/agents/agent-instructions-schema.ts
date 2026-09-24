import { z } from "zod";
import { agentInstructionsMaxLength, agentInstructionsTooLong } from "$lib/agents/instructions";

export const agentInstructionsSchema = z.object({
	instructions: z.string().trim().max(agentInstructionsMaxLength, agentInstructionsTooLong),
});

export type AgentInstructionsInput = z.infer<typeof agentInstructionsSchema>;
