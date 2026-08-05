import { z } from "zod";
import { grantSchema } from "$lib/account/mint-token-schema";

export const narrowConnectionSchema = z.object({
	capability: z.enum(["read", "write"]),
	followsMembership: z.boolean(),
	grants: z
		.array(grantSchema)
		.refine(
			(grants) => grants.every((grant) => grant.allTeams || grant.teamIds.length > 0),
			"Choose at least one team, or keep every team in that workspace."
		),
})
	.refine(
		(narrowed) => narrowed.followsMembership || narrowed.grants.length > 0,
		"Keep at least one workspace, or revoke the connection instead."
	);

export type NarrowConnectionInput = z.infer<typeof narrowConnectionSchema>;
