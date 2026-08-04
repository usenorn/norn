import { z } from "zod";

export const workspaceSettingsSchema = z.object({
	name: z.string().trim().min(1, "Enter a workspace name.").max(80, "Keep the name under 80 characters."),
	timezone: z.string().trim().min(1, "Choose a timezone."),
	defaultTeamId: z.string().trim().default(""),
});

export type WorkspaceSettingsInput = z.infer<typeof workspaceSettingsSchema>;
