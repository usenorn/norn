import { z } from "zod";
import { minPasswordLength } from "./sign-up-schema";

export const resetRequestSchema = z.object({
	email: z.email("Enter a valid email address."),
});

export const newPasswordSchema = z.object({
	password: z.string().min(minPasswordLength, `Use at least ${minPasswordLength} characters.`),
});

export type ResetRequestInput = z.infer<typeof resetRequestSchema>;
export type NewPasswordInput = z.infer<typeof newPasswordSchema>;
