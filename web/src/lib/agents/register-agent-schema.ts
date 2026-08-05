import { z } from "zod";

export const registerAgentSchema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Name this agent.")
		.max(80, "Keep the name under 80 characters."),
	ownerAccountId: z.string().min(1, "Choose who this agent acts for."),
	scopes: z.array(z.string()).min(1, "Choose at least one permission."),
	allTeams: z.boolean(),
	teamIds: z.array(z.string()),
	actionLimit: z.number().int().min(1).max(6000),
});

export type RegisterAgentInput = z.infer<typeof registerAgentSchema>;
