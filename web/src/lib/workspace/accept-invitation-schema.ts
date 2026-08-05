import { z } from "zod";
import { minPasswordLength } from "$lib/auth/sign-up-schema";

export const acceptInvitationSchema = z
	.object({
		name: z.string().trim().min(2, "Enter your name."),
		password: z.string().min(minPasswordLength, `Use at least ${minPasswordLength} characters.`),
		repeat: z.string(),
		terms: z.boolean().refine((agreed) => agreed, "Agree to the terms before joining."),
	})
	.refine((entered) => entered.password === entered.repeat, {
		message: "Passwords do not match.",
		path: ["repeat"],
	});

export type AcceptInvitationInput = z.infer<typeof acceptInvitationSchema>;
