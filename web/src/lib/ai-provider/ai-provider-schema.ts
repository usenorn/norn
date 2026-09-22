import { z } from "zod";

export const aiProviderKeyMaxLength = 512;
export const aiProviderModelMaxLength = 128;
export const aiProviderBaseUrlMaxLength = 2048;

const endpointMessage = "Enter a full http or https address, for example https://gateway.example.com/v1.";

export const aiProviderSchema = z
	.object({
		provider: z.enum(["openai"]).default("openai"),
		baseUrl: z
			.string()
			.trim()
			.max(aiProviderBaseUrlMaxLength, "That address is too long.")
			.refine((value) => {
				if (!value) return true;

				try {
					const parsed = new URL(value);

					return (parsed.protocol === "https:" || parsed.protocol === "http:") && !parsed.username;
				} catch {
					return false;
				}
			}, endpointMessage)
			.default(""),
		allowPrivateAddress: z.boolean().default(false),
		apiKey: z
			.string()
			.trim()
			.max(aiProviderKeyMaxLength, "That is longer than any key the provider issues.")
			.default(""),
		defaultModel: z.string().trim().max(aiProviderModelMaxLength).default(""),
		keyStored: z.boolean().default(false),
	})
	.superRefine((value, ctx) => {
		if (value.keyStored || value.apiKey) return;

		ctx.addIssue({
			code: "custom",
			path: ["apiKey"],
			message: "Paste the API key from your provider account.",
		});
	});

export type AiProviderInput = z.infer<typeof aiProviderSchema>;
