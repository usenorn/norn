import { z } from "zod";
import type { components } from "$lib/api/dashboard.gen";

type TeamColor = components["schemas"]["TeamColor"];
type TeamEstimation = components["schemas"]["TeamEstimation"];

export const teamColors = [
	"neutral",
	"cyan",
	"blue",
	"violet",
	"orchid",
	"magenta",
] as const satisfies readonly TeamColor[];

export const teamEstimations = [
	"none",
	"points",
	"hours",
	"sizes",
] as const satisfies readonly TeamEstimation[];

export const teamSettingsSchema = z.object({
	name: z
		.string()
		.trim()
		.min(2, "Enter a team name.")
		.max(80, "Keep the name under 80 characters."),
	description: z.string().trim().max(500, "Keep the description under 500 characters."),
	icon: z.string().trim().max(40, "Keep the icon under 40 characters."),
	iconColor: z.enum(teamColors).default("neutral"),
	estimation: z.enum(teamEstimations).default("none"),
	visibility: z.enum(["public", "private"]).default("public"),
});

export type TeamSettingsInput = z.infer<typeof teamSettingsSchema>;
