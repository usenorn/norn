import { z } from "zod";

const email = z.email("Enter a valid email address.");

export const identifySchema = z.object({ email });

export const signInSchema = z.object({
	email,
	password: z.string().min(1, "Enter your password."),
});

export type IdentifyInput = z.infer<typeof identifySchema>;

export type SignInInput = z.infer<typeof signInSchema>;
