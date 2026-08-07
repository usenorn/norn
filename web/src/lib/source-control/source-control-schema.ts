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
	baseUrl: z
		.union([z.literal(""), z.url("Enter the full address, including https://")])
		.optional(),
	label: z.string().trim().max(80, "Keep the label under 80 characters.").optional(),
	token,
});

export const addRepositorySchema = z.object({
	connectionId: z.string().min(1, "Choose a connection."),
	fullName: repository,
	mirrorLabel: z
		.string()
		.trim()
		.max(80, "Keep the label under 80 characters.")
		.optional(),
	pollIntervalSeconds: z.coerce
		.number()
		.int()
		.min(60, "Sweep at most once a minute.")
		.max(86400, "Sweep at least once a day.")
		.optional(),
});

export const mapIdentitySchema = z.object({
	accountId: z.string().min(1, "Choose a member."),
	provider: z.enum(["github", "gitlab"], { error: "Choose a platform." }),
	login: z
		.string()
		.trim()
		.min(1, "Give their handle on the platform.")
		.max(100, "That is longer than any handle a platform issues.")
		.regex(/^@?[^\s/]+$/, "A handle has no spaces or slashes."),
});

export const addRouteSchema = z.object({
	teamId: z.string().min(1, "Choose a team."),
	pathPrefix: z
		.string()
		.trim()
		.max(300, "Keep the path under 300 characters.")
		.refine((value) => !value.includes(".."), "A path cannot step outside the repository.")
		.optional(),
});

export const setTransitionRuleSchema = z.object({
	trigger: z.enum([
		"draft",
		"open",
		"review_requested",
		"changes_requested",
		"approved",
		"merged",
		"closed",
		"reopened",
		"conflicted",
	]),
	stateId: z.string().min(1, "Choose a state."),
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
	repositoryId: z.string().min(1, "Choose a connected repository."),
	reference: z
		.string()
		.trim()
		.min(1, "Give the number of the issue on the platform.")
		.regex(/^#?\d+$/, "Give the number of the issue, such as 41."),
});

export type ConnectSourceControlInput = z.infer<typeof connectSourceControlSchema>;
export type AddRepositoryInput = z.infer<typeof addRepositorySchema>;
export type AddRouteInput = z.infer<typeof addRouteSchema>;
export type MapIdentityInput = z.infer<typeof mapIdentitySchema>;
export type SetTransitionRuleInput = z.infer<typeof setTransitionRuleSchema>;
export type ReplaceTokenInput = z.infer<typeof replaceTokenSchema>;
export type LinkCodeInput = z.infer<typeof linkCodeSchema>;
export type MirrorIssueInput = z.infer<typeof mirrorIssueSchema>;
