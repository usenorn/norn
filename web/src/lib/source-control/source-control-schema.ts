import { z } from "zod";

const repository = z
	.string()
	.trim()
	.min(1, "Name the repository, as owner/name.")
	.max(200, "Keep the repository under 200 characters.")
	.regex(/^[A-Za-z0-9._-]+(\/[A-Za-z0-9._-]+)+$/, "Write it as owner/name, the way the platform does.");

const token = z
	.string()
	.trim()
	.min(1, "Paste a personal access token.")
	.max(500, "That is longer than any token a platform issues.");

export const connectSourceControlSchema = z.object({
	provider: z.enum(["github", "gitlab"], { error: "Choose a platform." }),
	repository,
	baseUrl: z
		.union([z.literal(""), z.url("Enter the full address, including https://")])
		.optional(),
	teamId: z.string().optional(),
	mirrorLabel: z
		.string()
		.trim()
		.max(80, "Keep the label under 80 characters.")
		.optional(),
	token,
});

export const replaceTokenSchema = z.object({ token });

export const linkCodeSchema = z.object({
	url: z
		.url("Paste the address of a branch, commit, pull request or merge request.")
		.refine(
			(value) => value.startsWith("https://") || value.startsWith("http://"),
			"The address has to be an http or https one.",
		),
});

export const mirrorIssueSchema = z.object({
	connectionId: z.string().min(1, "Choose a connected repository."),
	reference: z
		.string()
		.trim()
		.min(1, "Give the number of the issue on the platform.")
		.regex(/^#?\d+$/, "Give the number of the issue, such as 41."),
});

export type ConnectSourceControlInput = z.infer<typeof connectSourceControlSchema>;
export type ReplaceTokenInput = z.infer<typeof replaceTokenSchema>;
export type LinkCodeInput = z.infer<typeof linkCodeSchema>;
export type MirrorIssueInput = z.infer<typeof mirrorIssueSchema>;
