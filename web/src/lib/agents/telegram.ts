import type { components, operations } from "$lib/api/dashboard.gen";
import { workspacePath } from "$lib/workspace/navigation";

export type AgentTelegramBot = components["schemas"]["AgentTelegramBot"];
export type TelegramRefusal = components["schemas"]["TelegramRefusal"];
export type TelegramLinkPurpose = components["schemas"]["TelegramLinkPurpose"];
export type TelegramLinkInvite = components["schemas"]["TelegramLinkInvite"];
export type MemberTelegramBot = components["schemas"]["MemberTelegramBot"];

type ProblemOf<Responses> = Responses[keyof Responses] extends infer Response
	? Response extends { content: { "application/problem+json": infer Problem } }
		? Problem
		: never
	: never;

export type TelegramProblem =
	| ProblemOf<operations["connectWorkspaceAgentTelegram"]["responses"]>
	| ProblemOf<operations["disconnectWorkspaceAgentTelegram"]["responses"]>
	| ProblemOf<operations["createWorkspaceAgentTelegramLink"]["responses"]>
	| ProblemOf<operations["unlinkWorkspaceAgentTelegram"]["responses"]>
	| ProblemOf<operations["unbindWorkspaceAgentTelegramGroup"]["responses"]>;

export type TelegramPanel =
	| { kind: "loading" }
	| { kind: "disconnected" }
	| { kind: "connected"; bot: AgentTelegramBot }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type TelegramFailure =
	| { kind: "refused"; code: TelegramRefusal }
	| { kind: "sealing_unavailable" }
	| { kind: "disabled" }
	| { kind: "forbidden" }
	| { kind: "missing" }
	| { kind: "unavailable" };

export type TelegramOutcome =
	| { kind: "idle" }
	| { kind: "connected" }
	| { kind: "disconnected" }
	| { kind: "invited"; purpose: TelegramLinkPurpose; invite: TelegramLinkInvite }
	| { kind: "unlinked" }
	| { kind: "unbound"; title: string }
	| { kind: "failed"; failure: TelegramFailure };

export type MemberTelegramBots =
	| { kind: "loading" }
	| { kind: "none" }
	| { kind: "listed"; bots: MemberTelegramBot[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

const refusals: TelegramRefusal[] = ["token_rejected", "bot_taken", "origin_insecure", "unreachable"];

function isRefusal(code: unknown): code is TelegramRefusal {
	return refusals.includes(code as TelegramRefusal);
}

export function telegramFailure(problem: TelegramProblem | undefined, status: number): TelegramFailure {
	if (status === 403) return { kind: "forbidden" };
	if (status === 404) return { kind: "missing" };

	if (problem && "code" in problem) {
		if (problem.code === "telegram_sealing_unavailable") return { kind: "sealing_unavailable" };
		if (problem.code === "agent_disabled") return { kind: "disabled" };
		if (isRefusal(problem.code)) return { kind: "refused", code: problem.code };
	}

	return { kind: "unavailable" };
}

export function telegramFailureTitle(failure: TelegramFailure): string {
	switch (failure.kind) {
		case "refused":
			return {
				token_rejected: "Telegram rejected this token",
				bot_taken: "That bot already works for another agent",
				origin_insecure: "Telegram cannot reach this instance",
				unreachable: "Telegram could not be reached",
			}[failure.code];
		case "sealing_unavailable":
			return "The token cannot be stored";
		case "disabled":
			return "This agent is disabled";
		case "forbidden":
			return "You cannot change this";
		case "missing":
			return "That is already gone";
		case "unavailable":
			return "Something went wrong";
	}
}

export function telegramFailureMessage(failure: TelegramFailure, selfHosted: boolean): string {
	switch (failure.kind) {
		case "refused":
			return {
				token_rejected: "Copy the token again from BotFather. A revoked token stops working at once.",
				bot_taken: "One bot serves one agent. Create another bot with BotFather for this one.",
				origin_insecure: selfHosted
					? "Telegram only delivers to an https address. Set NORN_APP_BASE_URL to this instance's public https origin, then connect again."
					: "Telegram only delivers to an https address, and this instance is not served over one.",
				unreachable: "Nothing was changed. Wait a moment and try again.",
			}[failure.code];
		case "sealing_unavailable":
			return selfHosted
				? "This instance has no encryption key. Set NORN_SECURITY_ENCRYPTION_KEY and restart it."
				: "This instance cannot store secrets right now. Try again later.";
		case "disabled":
			return "Enable the agent before connecting its bot.";
		case "forbidden":
			return "Only the agent's owner or a workspace administrator can change its bot.";
		case "missing":
			return "Reload to see where things stand.";
		case "unavailable":
			return "Check your connection and try again.";
	}
}

export function telegramHandle(username: string): string {
	return `@${username}`;
}

export function telegramSettingsPath(slug: string): string {
	return workspacePath(slug, "/settings/notifications#telegram");
}
