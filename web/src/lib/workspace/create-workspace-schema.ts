import { z } from "zod";

export const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
export const teamKeyPattern = /^[A-Z]{2,5}$/;

export const createWorkspaceSchema = z.object({
	name: z.string().trim().min(2, "Enter a workspace name."),
	slug: z
		.string()
		.trim()
		.min(2, "Enter an address.")
		.max(48, "Keep the address under 48 characters.")
		.regex(slugPattern, { error: "Lowercase letters, numbers and single hyphens." }),
	teamName: z.string().trim().min(2, "Enter a team name."),
	teamKey: z
		.string()
		.trim()
		.regex(teamKeyPattern, { error: "Two to five capital letters." }),
});

export type CreateWorkspaceInput = z.infer<typeof createWorkspaceSchema>;
