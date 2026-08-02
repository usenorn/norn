import { z } from "zod";

export const personalEmailDomain = /@(gmail|outlook|hotmail|yahoo|icloud|proton|live|aol)\./i;

export const minPasswordLength = 12;

export const signUpSchema = z
	.object({
		name: z.string().trim().min(2, "Enter your name."),
		email: z
			.email("Enter a valid email address.")
			.refine((value) => !personalEmailDomain.test(value), {
				error: "Use your work address, not a personal one.",
			}),
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
