import { z } from "zod";

export const connectSchema = z.object({
	site: z.url("Enter the full site address, including https://"),
	email: z.email("Enter a valid email address."),
	token: z.string().trim().min(1, "Paste the API token."),
});

export type ConnectInput = z.infer<typeof connectSchema>;
