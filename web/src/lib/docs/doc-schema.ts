import { z } from "zod";
import { docTitleMax } from "./docs";

export const docTitleSchema = z
	.string()
	.trim()
	.min(1, "Give the doc a title.")
	.max(docTitleMax, `Keep the title under ${docTitleMax} characters.`);

const audienceShape = {
	visibility: z.enum(["workspace", "project", "personal"]).default("workspace"),
	projectId: z.string().default(""),
};

const projectChosen = {
	path: ["projectId"],
	error: "Choose the project whose members will read it.",
};

function chosesProject(input: { visibility: string; projectId: string }): boolean {
	return input.visibility !== "project" || input.projectId !== "";
}

export const newDocSchema = z
	.object({ title: docTitleSchema, parentId: z.string().default(""), ...audienceShape })
	.refine(chosesProject, projectChosen);

export const shareDocSchema = z.object(audienceShape).refine(chosesProject, projectChosen);

export type NewDocInput = z.infer<typeof newDocSchema>;
export type ShareDocInput = z.infer<typeof shareDocSchema>;

const expectedVersion = z.coerce.number().int().min(1);

export const archiveDocSchema = z.object({ expectedVersion });

export const restoreDocSchema = z.object({ revisionId: z.string().min(1), expectedVersion });

export const newDocFormId = "doc-new-form";
export const archiveFormId = "doc-archive-form";
export const restoreFormId = "doc-restore-form";
export const shareFormId = "doc-share-form";

export const docTitleFormSchema = z.object({ title: docTitleSchema });

export const docTitleFormId = "doc-title-form";
