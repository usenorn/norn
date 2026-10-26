import { z } from "zod";
import { agentAuthorityShape, refineTeamReach } from "./agent-authority-schema";
import { agentIcons, agentScopes } from "./agents";

export const defaultAgentActionLimit = 120;

export const registerAgentSchema = z
	.object({
		name: z
			.string()
			.trim()
			.min(1, "Name this agent.")
			.max(80, "Keep the name under 80 characters."),
		icon: z.enum(agentIcons).default("bot"),
		...agentAuthorityShape,
		scope: z.enum(agentScopes).default("member"),
		projectId: z.string().default(""),
		actionLimit: z.number().int().min(1).max(6000).default(defaultAgentActionLimit),
	})
	.superRefine((registration, context) => {
		refineTeamReach(registration, context);

		if (registration.scope === "project" && registration.projectId === "") {
			context.addIssue({
				code: "custom",
				path: ["projectId"],
				message: "Choose the project whose members may hand this agent work.",
			});
		}
	});

export type RegisterAgentInput = z.infer<typeof registerAgentSchema>;
