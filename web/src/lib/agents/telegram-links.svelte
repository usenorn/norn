<script lang="ts">
	import { invalidate } from "$app/navigation";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Send from "@lucide/svelte/icons/send";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { onDateAndTime } from "$lib/time";
	import {
		telegramFailure,
		telegramFailureMessage,
		telegramFailureTitle,
		telegramHandle,
		type MemberTelegramBots,
		type TelegramOutcome,
	} from "./telegram";

	let {
		workspaceId,
		bots,
		selfHosted,
		timezone,
		opened,
	}: {
		workspaceId: string;
		bots: MemberTelegramBots;
		selfHosted: boolean;
		timezone: string;
		opened?: TelegramOutcome;
	} = $props();

	let liveOutcome = $state<TelegramOutcome>({ kind: "idle" });
	let working = $state<string | null>(null);

	const outcome = $derived<TelegramOutcome>(opened ?? liveOutcome);

	async function link(agentId: string) {
		working = agentId;
		liveOutcome = { kind: "idle" };

		try {
			const { data, error, response } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/telegram/links",
				{ params: { path: { workspaceId, agentId } }, body: { purpose: "private" } }
			);

			liveOutcome = data
				? { kind: "invited", purpose: "private", invite: data }
				: { kind: "failed", failure: telegramFailure(error, response.status) };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			working = null;
		}
	}

	async function unlink(agentId: string) {
		working = agentId;
		liveOutcome = { kind: "idle" };

		try {
			const { error, response } = await api.DELETE(
				"/workspaces/{workspaceId}/agents/{agentId}/telegram/links/me",
				{ params: { path: { workspaceId, agentId } } }
			);

			if (error) {
				liveOutcome = { kind: "failed", failure: telegramFailure(error, response.status) };

				return;
			}

			await invalidate(keys.telegramBots(workspaceId));
			liveOutcome = { kind: "unlinked" };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			working = null;
		}
	}
</script>

<section id="telegram" class="flex flex-col gap-3" aria-labelledby="telegram-heading">
	<div class="flex flex-col gap-1">
		<h2 id="telegram-heading" class="text-md font-medium tracking-snug text-ink-900">Telegram</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Link your Telegram to an agent's bot and the questions it asks on work you handed it reach you there.
			Answer with a tap or a reply.
		</p>
	</div>

	<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
		{#if outcome.kind === "invited"}
			<Alert.Root variant="success">
				<Send aria-hidden="true" />
				<Alert.Title>Open the bot in Telegram</Alert.Title>
				<Alert.Description>
					Press Start in the chat that opens. The link works once and expires {onDateAndTime(
						outcome.invite.expiresAt,
						timezone
					)}.
				</Alert.Description>
				<Alert.Action>
					<Button href={outcome.invite.url} target="_blank" rel="noopener noreferrer" size="sm" variant="secondary">
						<ExternalLink aria-hidden="true" />
						Open Telegram
					</Button>
				</Alert.Action>
			</Alert.Root>
		{:else if outcome.kind === "unlinked"}
			<Alert.Root>
				<CircleCheck aria-hidden="true" />
				<Alert.Title>Unlinked</Alert.Title>
				<Alert.Description>That agent's questions no longer reach your Telegram.</Alert.Description>
			</Alert.Root>
		{:else if outcome.kind === "failed"}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>{telegramFailureTitle(outcome.failure)}</Alert.Title>
				<Alert.Description>{telegramFailureMessage(outcome.failure, selfHosted)}</Alert.Description>
			</Alert.Root>
		{/if}
	</div>

	{#if bots.kind === "loading"}
		<div class="flex flex-col gap-2" aria-busy="true" aria-label="Loading Telegram bots">
			<Skeleton class="h-12 w-full" />
			<Skeleton class="h-12 w-full" />
		</div>
	{:else if bots.kind === "forbidden"}
		<p class="text-sm text-muted-foreground">Viewers do not receive agents' questions.</p>
	{:else if bots.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Could not load the bots</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{:else if bots.kind === "none"}
		<p class="text-sm text-muted-foreground">
			No agent in this workspace has a Telegram bot yet. An agent's owner connects one on the agent's page.
		</p>
	{:else}
		<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
			{#each bots.bots as bot (bot.agentId)}
				<li class="flex flex-col gap-2 p-2.5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
					<div class="min-w-0">
						<p class="truncate text-sm text-ink-900">{bot.agentName}</p>
						<p class="truncate text-xs text-muted-foreground">
							{telegramHandle(bot.username)} · {bot.linked
								? bot.linkedAs
									? `linked as @${bot.linkedAs}`
									: "linked"
								: "not linked"}
						</p>
					</div>
					{#if bot.linked}
						<Button
							variant="secondary"
							size="sm"
							class="self-start sm:self-auto"
							disabled={working !== null}
							onclick={() => unlink(bot.agentId)}
						>
							{working === bot.agentId ? "Unlinking…" : "Unlink"}
						</Button>
					{:else}
						<Button
							size="sm"
							class="self-start sm:self-auto"
							disabled={working !== null}
							onclick={() => link(bot.agentId)}
						>
							{working === bot.agentId ? "Creating a link…" : "Link Telegram"}
						</Button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
