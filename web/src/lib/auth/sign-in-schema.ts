import { z } from "zod";

export const signInSchema = z.object({
	email: z.email("Enter a valid email address."),
	password: z.string().min(1, "Enter your password."),
});

export type SignInInput = z.infer<typeof signInSchema>;
