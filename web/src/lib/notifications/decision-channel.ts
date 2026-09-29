import type { components } from "$lib/api/dashboard.gen";
import type { MemberTelegramBots } from "$lib/agents/telegram";

export type DecisionChannel = components["schemas"]["DecisionChannel"];

export const decisionChannels = ["norn", "telegram"] as const satisfies readonly DecisionChannel[];

export const decisionChannelLabels: Record<DecisionChannel, string> = {
	norn: "Norn only",
	telegram: "Telegram",
};

export const decisionChannelHints: Record<DecisionChannel, string> = {
	norn: "Questions, plans and changes wait for you in Reviews.",
	telegram:
		"Each one also arrives as a message from the agent's bot, where you can answer or approve it. Reviews still lists them.",
};

export type DecisionChannelOutcome = { kind: "saved" } | { kind: "failed" };

export function telegramLinked(bots: MemberTelegramBots): boolean {
	return bots.kind === "listed" && bots.bots.some((bot) => bot.linked);
}
