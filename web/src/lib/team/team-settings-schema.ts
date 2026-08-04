import { z } from "zod";

export const teamSettingsSchema = z.object({
	name: z
		.string()
		.trim()
		.min(2, "Enter a team name.")
		.max(80, "Keep the name under 80 characters."),
	visibility: z.enum(["public", "private"]).default("public"),
});

export type TeamSettingsInput = z.infer<typeof teamSettingsSchema>;
