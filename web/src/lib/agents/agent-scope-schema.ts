import { z } from "zod";
import { agentScopes } from "./agents";

export const agentScopeSchema = z
	.object({
		scope: z.enum(agentScopes),
		projectId: z.string().default(""),
	})
	.superRefine((chosen, context) => {
		if (chosen.scope === "project" && chosen.projectId === "") {
			context.addIssue({
				code: "custom",
				path: ["projectId"],
				message: "Choose the project whose members may hand this agent work.",
			});
		}
	});

export type AgentScopeInput = z.infer<typeof agentScopeSchema>;
