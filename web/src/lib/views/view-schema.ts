import { z } from "zod";

export const viewSchema = z
	.object({
		name: z
			.string()
			.trim()
			.min(1, "Name this view.")
			.max(80, "Keep the name under 80 characters."),
		sharing: z.enum(["personal", "team", "workspace"]).default("personal"),
		teamId: z.string().default(""),
	})
	.refine((input) => input.sharing !== "team" || input.teamId !== "", {
		path: ["teamId"],
		error: "Choose the team to share it with.",
	});

export type ViewInput = z.infer<typeof viewSchema>;
