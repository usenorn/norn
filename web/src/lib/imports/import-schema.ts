import { z } from "zod";

const unknownReferences = z.enum(["skip", "create", "fail"]);

export const startImportSchema = z.object({
	sourceKind: z.string().min(1, "Choose where the backlog is coming from."),
	sourceLabel: z.string().trim().max(120, "Keep the name under 120 characters."),
});

export const linearKeySchema = z.object({
	apiKey: z
		.string()
		.trim()
		.min(1, "Paste a Linear personal API key.")
		.max(512, "That is longer than any Linear key."),
	unknownReferences,
});

export const csvFileSchema = z.object({
	file: z
		.instanceof(File, { message: "Choose a file to read the rows from." })
		.nullable()
		.refine((chosen) => chosen !== null, "Choose a file to read the rows from."),
	unknownReferences,
});

export const linearScopeSchema = z.object({
	teamIds: z.array(z.string()).min(1, "Choose at least one team to read."),
});

export const csvShapeSchema = z.object({
	delimiter: z.string(),
	header: z.boolean(),
	teamKey: z
		.string()
		.trim()
		.regex(/^[A-Z]{2,5}$/, "Two to five capital letters, such as OPS."),
	teamName: z
		.string()
		.trim()
		.min(1, "Name the team these rows stand for.")
		.max(80, "Keep the name under 80 characters."),
	columns: z.array(z.object({ index: z.number().int(), target: z.string() })),
});

export const mappingSchema = z.object({
	decisions: z
		.array(
			z.object({
				kind: z.enum(["user", "state", "priority", "label", "project", "team"]),
				sourceKey: z.string(),
				decision: z.enum(["", "map", "create", "unattributed", "skip"]),
				targetId: z.string(),
				targetValue: z.string(),
			})
		)
		.refine(
			(decisions) =>
				decisions.every(
					(decision) =>
						decision.decision !== "map" || decision.targetId !== "" || decision.targetValue !== ""
				),
			"Every concept set to use one already here needs something chosen."
		),
});

export const executeSchema = z.object({
	previewDigest: z.string().min(1),
	acknowledgeTriage: z.boolean(),
});

export type StartImportInput = z.infer<typeof startImportSchema>;
export type LinearKeyInput = z.infer<typeof linearKeySchema>;
export type CsvFileInput = z.infer<typeof csvFileSchema>;
export type LinearScopeInput = z.infer<typeof linearScopeSchema>;
export type CsvShapeInput = z.infer<typeof csvShapeSchema>;
export type MappingInput = z.infer<typeof mappingSchema>;
export type ExecuteInput = z.infer<typeof executeSchema>;
