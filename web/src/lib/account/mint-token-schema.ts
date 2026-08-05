import { z } from "zod";

export const grantSchema = z.object({
	workspaceId: z.string().min(1),
	allTeams: z.boolean(),
	teamIds: z.array(z.string()),
});

export const mintTokenSchema = z.object({
	name: z.string().trim().min(1, "Name this token.").max(80, "Keep the name under 80 characters."),
	scopes: z.array(z.string()).min(1, "Choose at least one permission."),
	grants: z
		.array(grantSchema)
		.min(1, "Choose at least one workspace for this token to reach.")
		.refine(
			(grants) => grants.every((grant) => grant.allTeams || grant.teamIds.length > 0),
			"Choose at least one team, or give the token every team in that workspace."
		),
	expiresInDays: z.number().int().min(1).max(365),
});

export type MintTokenInput = z.infer<typeof mintTokenSchema>;
export type GrantInput = z.infer<typeof grantSchema>;
