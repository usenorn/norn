import { z } from "zod";

export const domainSchema = z.object({
	domain: z.string().trim().min(1, "Enter a domain like northwind.co."),
});

export type DomainInput = z.infer<typeof domainSchema>;
