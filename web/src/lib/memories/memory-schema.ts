import { z } from "zod";
import { audienceShape, chosesProject, projectChosen } from "#lib/docs/doc-schema.js";
import {
	memoryComponentMax,
	memoryKinds,
	memoryProseMax,
	memoryReasonMax,
	memoryStatementMax,
	wholeWorkspace,
} from "./memories";

const day = z.string().regex(/^(\d{4}-\d{2}-\d{2})?$/, "Pick a day.").default("");

const contentShape = {
	kind: z.enum(memoryKinds).default("decision"),
	statement: z
		.string()
		.trim()
		.min(1, "Say what the memory holds.")
		.max(memoryStatementMax, `Keep it under ${memoryStatementMax} characters.`),
	rationale: z.string().trim().max(memoryProseMax, `Keep it under ${memoryProseMax} characters.`).default(""),
	conditions: z.string().trim().max(memoryProseMax, `Keep it under ${memoryProseMax} characters.`).default(""),
	appliesTo: z.string().min(1).default(wholeWorkspace),
	component: z.string().trim().max(memoryComponentMax, `Keep it under ${memoryComponentMax} characters.`).default(""),
	basis: z.enum(["observation", "inference"]).default("observation"),
	effectiveFrom: day,
	effectiveUntil: day,
	expectedVersion: z.coerce.number().int().min(1),
};

function endsAfterItStarts(input: { effectiveFrom: string; effectiveUntil: string }): boolean {
	return !input.effectiveFrom || !input.effectiveUntil || input.effectiveUntil >= input.effectiveFrom;
}

const untilAfterFrom = { path: ["effectiveUntil"], error: "End it on or after the day it starts." };

export const correctMemorySchema = z.object(contentShape).refine(endsAfterItStarts, untilAfterFrom);

export const supersedeMemorySchema = z
	.object({ ...contentShape, ...audienceShape })
	.refine(endsAfterItStarts, untilAfterFrom)
	.refine(chosesProject, projectChosen);

export const rejectMemorySchema = z.object({
	reason: z
		.string()
		.trim()
		.min(1, "Say why, so the author can learn from it.")
		.max(memoryReasonMax, `Keep it under ${memoryReasonMax} characters.`),
});

export type CorrectMemoryInput = z.infer<typeof correctMemorySchema>;
export type SupersedeMemoryInput = z.infer<typeof supersedeMemorySchema>;

export const correctMemoryFormId = "memory-correct-form";
export const supersedeMemoryFormId = "memory-supersede-form";
export const rejectMemoryFormId = "memory-reject-form";
export const shareMemoryFormId = "memory-share-form";
