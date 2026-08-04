import { z } from "zod";

export const labelSchema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Enter a label name.")
		.max(40, "Keep the name under 40 characters."),
	color: z.enum(["neutral", "cyan", "blue", "violet", "orchid", "magenta"]).default("cyan"),
	groupId: z.string().default(""),
	teamId: z.string().default(""),
});

export type LabelInput = z.infer<typeof labelSchema>;

export const labelGroupSchema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Enter a group name.")
		.max(40, "Keep the name under 40 characters."),
});

export type LabelGroupInput = z.infer<typeof labelGroupSchema>;
