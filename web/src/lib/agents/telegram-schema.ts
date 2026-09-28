import { z } from "zod";

export const telegramTokenMaxLength = 128;

export const telegramConnectFormId = "agent-telegram-connect";

export const telegramConnectSchema = z.object({
	workspaceId: z.string().min(1),
	agentId: z.string().min(1),
	token: z
		.string()
		.trim()
		.min(1, "Paste the token BotFather gave you.")
		.max(telegramTokenMaxLength, "That is longer than any token BotFather issues.")
		.regex(/^[0-9]{1,20}:[A-Za-z0-9_-]{30,64}$/, "That does not look like a BotFather token."),
});

export type TelegramConnectInput = z.infer<typeof telegramConnectSchema>;
