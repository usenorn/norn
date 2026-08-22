import { z } from "zod";

export const signInCodeLength = 6;

export const signInCodeSchema = z.object({
	challengeId: z.string().min(1),
	code: z
		.string()
		.trim()
		.transform((typed) => typed.replaceAll(" ", "").replaceAll("-", ""))
		.pipe(
			z
				.string()
				.regex(/^[0-9]+$/, "The code is six digits.")
				.length(signInCodeLength, "The code is six digits.")
		),
});

export type SignInCodeInput = z.infer<typeof signInCodeSchema>;
