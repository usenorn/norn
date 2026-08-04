import { z } from "zod";
import { minPasswordLength } from "$lib/auth/sign-up-schema";

export const acceptInvitationSchema = z.object({
	name: z.string().trim().min(2, "Enter your name."),
	password: z.string().min(minPasswordLength, `Use at least ${minPasswordLength} characters.`),
});

export type AcceptInvitationInput = z.infer<typeof acceptInvitationSchema>;
