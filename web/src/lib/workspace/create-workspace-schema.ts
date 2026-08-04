import { z } from "zod";
import { teamKeyPattern } from "$lib/team/teams";

export const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export const createWorkspaceSchema = z.object({
	name: z.string().trim().min(2, "Enter a workspace name."),
	slug: z
		.string()
		.trim()
		.min(2, "Enter an address.")
		.max(48, "Keep the address under 48 characters.")
		.regex(slugPattern, { error: "Lowercase letters, numbers and single hyphens." }),
	teamName: z
		.string()
		.trim()
		.min(2, "Enter a name for your first team.")
		.max(80, "Keep the name under 80 characters."),
	teamKey: z
		.string()
		.trim()
		.toUpperCase()
		.regex(teamKeyPattern, { error: "Two to five letters, A–Z." }),
});

export type CreateWorkspaceInput = z.infer<typeof createWorkspaceSchema>;

export function slugFromName(name: string): string {
	return name
		.normalize("NFD")
		.replace(/\p{Diacritic}/gu, "")
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/^-+/, "")
		.slice(0, 48)
		.replace(/-+$/, "");
}

export function slugSuggestions(slug: string): string[] {
	return ["co", "hq", "team"].map((suffix) => `${slug}-${suffix}`);
}

export function slugMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter an address.";
		case "too_short":
			return "Use at least 2 characters.";
		case "too_long":
			return "Keep the address under 48 characters.";
		case "malformed":
			return "Lowercase letters, numbers and single hyphens.";
		default:
			return "That address cannot be used.";
	}
}
