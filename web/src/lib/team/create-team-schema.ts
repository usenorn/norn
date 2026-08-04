import { z } from "zod";
import { teamKeyPattern } from "./teams";

export const createTeamSchema = z.object({
	name: z
		.string()
		.trim()
		.min(2, "Enter a team name.")
		.max(80, "Keep the name under 80 characters."),
	key: z
		.string()
		.trim()
		.toUpperCase()
		.min(2, "Use at least 2 letters.")
		.max(5, "Use at most 5 letters.")
		.regex(teamKeyPattern, { error: "Two to five letters, A–Z." }),
	visibility: z.enum(["public", "private"]).default("public"),
});

export type CreateTeamInput = z.infer<typeof createTeamSchema>;
