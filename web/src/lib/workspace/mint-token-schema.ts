import { z } from "zod";

export const mintTokenSchema = z.object({
	name: z.string().trim().min(1, "Name this token.").max(80, "Keep the name under 80 characters."),
	scopes: z.array(z.string()).min(1, "Choose at least one permission."),
});

export type MintTokenInput = z.infer<typeof mintTokenSchema>;
