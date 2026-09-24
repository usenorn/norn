import { z } from "zod";
import { agentIcons, agentPermissions, agentScopes } from "./agents";

export const defaultAgentActionLimit = 120;

export const registerAgentSchema = z
	.object({
		name: z
			.string()
			.trim()
			.min(1, "Name this agent.")
			.max(80, "Keep the name under 80 characters."),
		icon: z.enum(agentIcons).default("bot"),
		scopes: z
			.array(z.enum(agentPermissions))
			.min(1, "Choose at least one permission.")
			.prefault([]),
		scope: z.enum(agentScopes).default("member"),
		projectId: z.string().default(""),
		allTeams: z.boolean().default(true),
		teamIds: z.array(z.string()).default([]),
		actionLimit: z.number().int().min(1).max(6000).default(defaultAgentActionLimit),
	})
	.superRefine((registration, context) => {
		if (!registration.allTeams && registration.teamIds.length === 0) {
			context.addIssue({
				code: "custom",
				path: ["allTeams"],
				message: "Choose at least one team, or give the agent every reachable team.",
			});
		}

		if (registration.scope === "project" && registration.projectId === "") {
			context.addIssue({
				code: "custom",
				path: ["projectId"],
				message: "Choose the project whose members may hand this agent work.",
			});
		}
	});

export type RegisterAgentInput = z.infer<typeof registerAgentSchema>;
