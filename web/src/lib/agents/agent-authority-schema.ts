import { z } from "zod";
import { agentPermissions } from "./agents";

export const agentAuthorityShape = {
	scopes: z
		.array(z.enum(agentPermissions))
		.min(1, "Choose at least one permission.")
		.prefault([]),
	allTeams: z.boolean().default(true),
	teamIds: z.array(z.string()).default([]),
};

export function refineTeamReach(
	authority: { allTeams: boolean; teamIds: string[] },
	context: z.RefinementCtx
) {
	if (!authority.allTeams && authority.teamIds.length === 0) {
		context.addIssue({
			code: "custom",
			path: ["allTeams"],
			message: "Choose at least one team, or give the agent every reachable team.",
		});
	}
}

export const agentAuthoritySchema = z.object(agentAuthorityShape).superRefine(refineTeamReach);

export type AgentAuthorityInput = z.infer<typeof agentAuthoritySchema>;
