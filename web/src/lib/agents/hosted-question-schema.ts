import { z } from "zod";
import { conversationTurnLength } from "./hosted";

export const hostedQuestionSchema = z.object({
	question: z
		.string()
		.trim()
		.min(1, "Ask the agent something.")
		.max(conversationTurnLength, `Keep a question under ${conversationTurnLength.toLocaleString("en")} characters.`),
});

export type HostedQuestionInput = z.infer<typeof hostedQuestionSchema>;
