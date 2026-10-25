import { z } from "zod";
import type { components } from "#lib/api/dashboard.gen.js";
import { roadmapDescriptionMaxLength, roadmapTitleMaxLength } from "./roadmap-settings-schema";

type RoadmapPublicStatus = components["schemas"]["RoadmapPublicStatus"];
type RoadmapProgressMode = components["schemas"]["RoadmapProgressMode"];

export const roadmapStatuses = [
	"planned",
	"in_progress",
	"paused",
	"completed",
	"cancelled",
] as const satisfies readonly RoadmapPublicStatus[];

export const roadmapProgressModes = [
	"hidden",
	"approved",
	"calculated",
] as const satisfies readonly RoadmapProgressMode[];

export const roadmapProgressMax = 100;

const calendarDate = z
	.string()
	.trim()
	.refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value), {
		message: "Use a date like 2026-09-30.",
	});

export const roadmapEntrySchema = z
	.object({
		publicTitle: z
			.string()
			.trim()
			.min(1, "Give this entry a public title.")
			.max(roadmapTitleMaxLength, `Keep the public title under ${roadmapTitleMaxLength} characters.`),
		publicDescription: z
			.string()
			.trim()
			.max(
				roadmapDescriptionMaxLength,
				`Keep the public description under ${roadmapDescriptionMaxLength} characters.`
			),
		status: z.enum(roadmapStatuses).nullable(),
		timeline: z.object({ startOn: calendarDate, endOn: calendarDate }),
		progressMode: z.enum(roadmapProgressModes),
		progressPercent: z
			.number()
			.int("Use a whole number.")
			.min(0, "Use a number from 0 to 100.")
			.max(roadmapProgressMax, "Use a number from 0 to 100.")
			.nullable(),
		visibility: z.object({
			issueLists: z.boolean(),
			projectDescriptions: z.boolean(),
			issueDescriptions: z.boolean(),
			status: z.boolean(),
			timeline: z.boolean(),
			progress: z.boolean(),
		}),
	})
	.superRefine((entry, context) => {
		const { startOn, endOn } = entry.timeline;

		if (startOn !== "" && endOn === "") {
			context.addIssue({
				code: "custom",
				path: ["timeline", "endOn"],
				message: "Add an end date. A start date is only shown with one.",
			});
		}

		if (startOn !== "" && endOn !== "" && endOn < startOn) {
			context.addIssue({
				code: "custom",
				path: ["timeline", "endOn"],
				message: "End on or after the start date.",
			});
		}

		if (entry.progressMode === "approved" && entry.progressPercent === null) {
			context.addIssue({
				code: "custom",
				path: ["progressPercent"],
				message: "Enter the progress you approve for publication.",
			});
		}
	});

export type RoadmapEntryInput = z.infer<typeof roadmapEntrySchema>;
