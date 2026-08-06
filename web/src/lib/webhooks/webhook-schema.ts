import { z } from "zod";

const destination = z.url("Enter the full address Norn should post to, including https://");

const events = z.array(z.string()).min(1, "Choose at least one event to send.");

export const createWebhookSchema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Name this subscription.")
		.max(80, "Keep the name under 80 characters."),
	url: destination,
	events,
});

export const editWebhookSchema = z.object({
	url: destination,
	events,
});

export type CreateWebhookInput = z.infer<typeof createWebhookSchema>;
export type EditWebhookInput = z.infer<typeof editWebhookSchema>;
