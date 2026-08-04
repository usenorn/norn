import { z } from "zod";

export const minPasswordLength = 12;

export const signUpSchema = z
	.object({
		name: z.string().trim().min(2, "Enter your name."),
		email: z.email("Enter a valid email address."),
		password: z.string().min(minPasswordLength, `Use at least ${minPasswordLength} characters.`),
		passwordConfirm: z.string(),
		terms: z.boolean().refine((accepted) => accepted, {
			error: "Accept the terms to continue.",
		}),
	})
	.refine((data) => data.password === data.passwordConfirm, {
		error: "Passwords do not match.",
		path: ["passwordConfirm"],
	});

export type SignUpInput = z.infer<typeof signUpSchema>;
