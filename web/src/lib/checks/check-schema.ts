import { z } from "zod";

export const newCheckSchema = z.object({
	statement: z
		.string()
		.trim()
		.min(3, "Say what has to be true, as a claim")
		.max(300, "That is longer than a criterion should be"),
	method: z.enum(["command", "observation", "manual", "regression"]),
	proof: z
		.string()
		.trim()
		.min(1, "Say how this gets proven, so nobody is asked to verify something nothing produces")
		.max(2000, "That is longer than Norn will carry"),
});

export type NewCheckInput = z.infer<typeof newCheckSchema>;

export const checkReasonSchema = z.object({
	reason: z
		.string()
		.trim()
		.min(1, "Say why, so the record is worth reading later")
		.max(2000, "That is longer than Norn will carry"),
});

export type CheckReasonInput = z.infer<typeof checkReasonSchema>;

export const evidenceSchema = z.object({
	verdict: z.enum(["passed", "failed", "absent_negative", "inconclusive"]),
	channel: z.enum(["command", "http", "log", "screenshot", "database", "human"]),
	command: z.string().max(2000, "That is longer than Norn will carry").default(""),
	output: z
		.string()
		.min(1, "File what you actually saw, not a description of it")
		.max(65536, "Norn keeps 64 KiB of output; trim it to the part that proves the claim"),
});

export type EvidenceInput = z.infer<typeof evidenceSchema>;
