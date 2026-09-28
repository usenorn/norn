import { z } from "zod";
import { agentExecutions } from "./agents";

export const agentExecutionSchema = z.object({
	execution: z.enum(agentExecutions),
});

export type AgentExecutionInput = z.infer<typeof agentExecutionSchema>;
