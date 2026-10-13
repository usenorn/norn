import { z } from "zod";

export const roadmapTitleMaxLength = 120;
export const roadmapDescriptionMaxLength = 4000;

export const roadmapSettingsSchema = z.object({
	expectedVersion: z.number().int().min(1),
	title: z
		.string()
		.trim()
		.max(roadmapTitleMaxLength, `Keep the public title under ${roadmapTitleMaxLength} characters.`),
	description: z
		.string()
		.trim()
		.max(
			roadmapDescriptionMaxLength,
			`Keep the public description under ${roadmapDescriptionMaxLength} characters.`
		),
	issueLists: z.boolean(),
	projectDescriptions: z.boolean(),
	issueDescriptions: z.boolean(),
	status: z.boolean(),
	timeline: z.boolean(),
	progress: z.boolean(),
});

export type RoadmapSettingsInput = z.infer<typeof roadmapSettingsSchema>;
