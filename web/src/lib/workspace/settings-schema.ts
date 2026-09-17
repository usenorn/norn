import { z } from "zod";

export const workspaceSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export const workspaceSettingsSchema = z.object({
	name: z.string().trim().min(1, "Enter a workspace name.").max(80, "Keep the name under 80 characters."),
	slug: z
		.string()
		.trim()
		.toLowerCase()
		.min(2, "Use at least 2 characters.")
		.max(40, "Keep the identifier under 40 characters.")
		.regex(workspaceSlugPattern, "Lowercase letters, numbers and dashes. It is the workspace address."),
	timezone: z.string().trim().min(1, "Choose a timezone."),
	weekStartsOn: z.enum(["monday", "sunday"]),
	defaultTeamId: z.string().trim().default(""),
});

export type WorkspaceSettingsInput = z.infer<typeof workspaceSettingsSchema>;
