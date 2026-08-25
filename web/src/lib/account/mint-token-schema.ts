import { z } from "zod";
import type { components } from "$lib/api/dashboard.gen";

const apiScopes = [
	"workspace:read",
	"workspace:update",
	"membership:read",
	"membership:manage",
	"invitation:read",
	"invitation:manage",
	"team:read",
	"team:manage",
	"team_membership:read",
	"team_membership:manage",
	"issue:read",
	"issue:manage",
	"cycle:read",
	"label:read",
	"label:manage",
	"project:read",
	"project:manage",
	"comment:read",
	"comment:manage",
	"notification:read",
	"notification:manage",
	"audit_log:read",
] as const satisfies readonly components["schemas"]["APIScope"][];

export const grantSchema = z.object({
	workspaceId: z.string().min(1),
	allTeams: z.boolean(),
	teamIds: z.array(z.string()),
});

export const mintTokenSchema = z.object({
	name: z.string().trim().min(1, "Name this token.").max(80, "Keep the name under 80 characters."),
	scopes: z.array(z.enum(apiScopes)).min(1, "Choose at least one permission."),
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
