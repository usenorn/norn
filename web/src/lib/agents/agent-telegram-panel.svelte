<script lang="ts">
	import { invalidate } from "$app/navigation";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Send from "@lucide/svelte/icons/send";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { superForm, type Infer, type SuperValidated } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { onDateAndTime } from "$lib/time";
	import {
		telegramFailure,
		telegramFailureMessage,
		telegramFailureTitle,
		telegramHandle,
		telegramSettingsPath,
		type TelegramLinkPurpose,
		type TelegramOutcome,
		type TelegramPanel,
	} from "./telegram";
	import { telegramConnectFormId, telegramConnectSchema } from "./telegram-schema";

	let {
		workspace,
		agentId,
		agentName,
		hosted,
		active,
		panel,
		connectForm,
		selfHosted,
		timezone,
		opened,
	}: {
		workspace: { id: string; slug: string };
		agentId: string;
		agentName: string;
		hosted: boolean;
		active: boolean;
		panel: TelegramPanel;
		connectForm: SuperValidated<Infer<typeof telegramConnectSchema>, TelegramOutcome>;
		selfHosted: boolean;
		timezone: string;
		opened?: TelegramOutcome;
	} = $props();

	type Mutation = "link" | "group" | "unlink" | "disconnect" | { unbind: string };

	let liveOutcome = $state<TelegramOutcome>({ kind: "idle" });
	let mutation = $state<Mutation | null>(null);
	let confirmingDisconnect = $state(false);

	// svelte-ignore state_referenced_locally
	const form = superForm(connectForm, {
		id: telegramConnectFormId,
		validators: zod4Client(telegramConnectSchema),
		resetForm: true,
		onSubmit: () => {
			liveOutcome = { kind: "idle" };
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	$effect(() => {
		const posted = $message;

		if (posted) liveOutcome = posted;
	});

	const outcome = $derived<TelegramOutcome>(opened ?? liveOutcome);
	const bot = $derived(panel.kind === "connected" ? panel.bot : null);
	const busy = $derived(mutation !== null || $submitting);

	async function refresh() {
		await Promise.all([invalidate(keys.agentTelegram(workspace.id, agentId)), invalidate(keys.telegramBots(workspace.id))]);
	}

	async function invite(purpose: TelegramLinkPurpose) {
		mutation = purpose === "group" ? "group" : "link";
		liveOutcome = { kind: "idle" };

		try {
			const { data, error, response } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/telegram/links",
				{ params: { path: { workspaceId: workspace.id, agentId } }, body: { purpose } }
			);

			liveOutcome = data
				? { kind: "invited", purpose, invite: data }
				: { kind: "failed", failure: telegramFailure(error, response.status) };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			mutation = null;
		}
	}

	async function unlink() {
		mutation = "unlink";
		liveOutcome = { kind: "idle" };

		try {
			const { error, response } = await api.DELETE(
				"/workspaces/{workspaceId}/agents/{agentId}/telegram/links/me",
				{ params: { path: { workspaceId: workspace.id, agentId } } }
			);

			if (error) {
				liveOutcome = { kind: "failed", failure: telegramFailure(error, response.status) };

				return;
			}

			await refresh();
			liveOutcome = { kind: "unlinked" };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			mutation = null;
		}
	}

	async function unbind(groupId: string, title: string) {
		mutation = { unbind: groupId };
		liveOutcome = { kind: "idle" };

		try {
			const { error, response } = await api.DELETE(
				"/workspaces/{workspaceId}/agents/{agentId}/telegram/groups/{groupId}",
				{ params: { path: { workspaceId: workspace.id, agentId, groupId } } }
			);

			if (error) {
				liveOutcome = { kind: "failed", failure: telegramFailure(error, response.status) };

				return;
			}

			await refresh();
			liveOutcome = { kind: "unbound", title };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			mutation = null;
		}
	}

	async function disconnect() {
		mutation = "disconnect";
		liveOutcome = { kind: "idle" };

		try {
			const { error, response } = await api.DELETE("/workspaces/{workspaceId}/agents/{agentId}/telegram", {
				params: { path: { workspaceId: workspace.id, agentId } },
			});

			if (error) {
				liveOutcome = { kind: "failed", failure: telegramFailure(error, response.status) };

				return;
			}

			await refresh();
			liveOutcome = { kind: "disconnected" };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			mutation = null;
			confirmingDisconnect = false;
		}
	}

	function unbinding(groupId: string): boolean {
		return typeof mutation === "object" && mutation !== null && mutation.unbind === groupId;
	}
</script>

<div class="flex flex-col gap-4">
	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		{hosted
			? `${agentName} posts its questions to Telegram through its own bot, and people answer with a tap or a reply. Its owner and workspace administrators can also talk to it there.`
			: `${agentName} posts its questions to Telegram through its own bot, and people answer with a tap or a reply. It works through its runner, so it does not chat there.`}
	</p>

	<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
		{#if outcome.kind === "connected"}
			<Alert.Root variant="success">
				<CircleCheck aria-hidden="true" />
				<Alert.Title>Bot connected</Alert.Title>
				<Alert.Description>Link your own Telegram next, so the questions you delegate reach you.</Alert.Description>
			</Alert.Root>
		{:else if outcome.kind === "disconnected"}
			<Alert.Root>
				<CircleCheck aria-hidden="true" />
				<Alert.Title>Bot disconnected</Alert.Title>
				<Alert.Description>Norn no longer hears from it, and everyone linked to it was forgotten.</Alert.Description>
			</Alert.Root>
		{:else if outcome.kind === "invited"}
			<Alert.Root variant="success">
				<Send aria-hidden="true" />
				<Alert.Title>{outcome.purpose === "group" ? "Add the bot to a group" : "Open the bot in Telegram"}</Alert.Title>
				<Alert.Description>
					<span class="block">
						{outcome.purpose === "group"
							? "Choose the group in Telegram. The group is connected once your own linked account starts the bot there."
							: "Press Start in the chat that opens. The link works once."}
						It expires {onDateAndTime(outcome.invite.expiresAt, timezone)}.
					</span>
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
				<Alert.Description>{agentName}'s questions no longer reach your Telegram.</Alert.Description>
			</Alert.Root>
		{:else if outcome.kind === "unbound"}
			<Alert.Root>
				<CircleCheck aria-hidden="true" />
				<Alert.Title>Group disconnected</Alert.Title>
				<Alert.Description>{outcome.title || "The group"} no longer receives {agentName}'s questions.</Alert.Description>
			</Alert.Root>
		{:else if outcome.kind === "failed"}
			<Alert.Root variant="destructive">
				<TriangleAlert aria-hidden="true" />
				<Alert.Title>{telegramFailureTitle(outcome.failure)}</Alert.Title>
				<Alert.Description>{telegramFailureMessage(outcome.failure, selfHosted)}</Alert.Description>
			</Alert.Root>
		{/if}
	</div>

	{#if panel.kind === "loading"}
		<div class="flex flex-col gap-3" aria-busy="true" aria-label="Loading the Telegram bot">
			<Skeleton class="h-28 w-full" />
			<Skeleton class="h-40 w-full" />
		</div>
	{:else if panel.kind === "forbidden"}
		<Alert.Root variant="muted">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>You cannot see this bot</Alert.Title>
			<Alert.Description>Only the agent's owner and workspace administrators manage its bot.</Alert.Description>
		</Alert.Root>
	{:else if panel.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>Could not load the bot</Alert.Title>
			<Alert.Description>Check your connection and reload.</Alert.Description>
		</Alert.Root>
	{:else}
		{#if bot}
			<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4" aria-labelledby="telegram-bot">
				<div class="flex flex-col gap-1">
					<h2 id="telegram-bot" class="text-md font-medium tracking-snug text-ink-900">
						{telegramHandle(bot.username)}
					</h2>
					<p class="text-sm text-muted-foreground">
						{bot.name}{bot.connectedBy ? ` · connected by ${bot.connectedBy}` : ""} · {onDateAndTime(
							bot.connectedAt,
							timezone
						)}
					</p>
				</div>

				<dl class="flex flex-col gap-2 text-sm">
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Token</dt>
						<dd class="text-ink-900">{bot.tokenHint ? `Ending ${bot.tokenHint}` : "Stored"}</dd>
					</div>
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Your Telegram</dt>
						<dd class="text-ink-900">{bot.linked ? "Linked" : "Not linked"}</dd>
					</div>
				</dl>

				<div class="flex flex-wrap items-center gap-2">
					{#if bot.linked}
						<Button variant="secondary" size="sm" disabled={busy} onclick={unlink}>
							{mutation === "unlink" ? "Unlinking…" : "Unlink my Telegram"}
						</Button>
					{:else}
						<Button size="sm" disabled={busy || !active} onclick={() => invite("private")}>
							{mutation === "link" ? "Creating a link…" : "Link my Telegram"}
						</Button>
					{/if}
					<Button variant="secondary" size="sm" disabled={busy || !active} onclick={() => invite("group")}>
						{mutation === "group" ? "Creating a link…" : "Add to a group"}
					</Button>
					<Button variant="destructive" size="sm" disabled={busy} onclick={() => (confirmingDisconnect = true)}>
						Disconnect
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-2" aria-labelledby="telegram-groups">
				<h2 id="telegram-groups" class="text-sm font-medium tracking-snug text-ink-900">Groups</h2>
				{#if bot.groups.length === 0}
					<p class="text-sm text-muted-foreground">
						No group receives {agentName}'s questions yet. Add the bot to one with a link from here.
					</p>
				{:else}
					<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
						{#each bot.groups as group (group.id)}
							<li class="flex flex-col gap-2 p-2.5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
								<div class="min-w-0">
									<p class="truncate text-sm text-ink-900">{group.title || "Untitled group"}</p>
									<p class="text-xs text-muted-foreground">
										{group.boundBy ? `Connected by ${group.boundBy}` : "Connected"} · {onDateAndTime(
											group.boundAt,
											timezone
										)}
									</p>
								</div>
								<Button
									variant="secondary"
									size="sm"
									class="self-start sm:self-auto"
									disabled={busy}
									onclick={() => unbind(group.id, group.title)}
								>
									{unbinding(group.id) ? "Disconnecting…" : "Disconnect"}
								</Button>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="flex flex-col gap-2" aria-labelledby="telegram-members">
				<h2 id="telegram-members" class="text-sm font-medium tracking-snug text-ink-900">Linked people</h2>
				{#if bot.members.length === 0}
					<p class="text-sm text-muted-foreground">
						Nobody has linked their Telegram yet. Members link theirs from their
						<a href={telegramSettingsPath(workspace.slug)} class="text-link underline-offset-2 hover:underline"
							>notification settings</a
						>.
					</p>
				{:else}
					<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
						{#each bot.members as member (member.accountId)}
							<li class="flex flex-col gap-0.5 p-2.5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
								<span class="truncate text-sm text-ink-900">{member.name}</span>
								<span class="text-xs text-muted-foreground">
									{member.username ? `@${member.username} · ` : ""}{onDateAndTime(member.linkedAt, timezone)}
								</span>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		{/if}

		{#if !active}
			<Alert.Root variant="muted">
				<CircleAlert aria-hidden="true" />
				<Alert.Title>This agent is disabled</Alert.Title>
				<Alert.Description>Enable it to connect a bot or create links.</Alert.Description>
			</Alert.Root>
		{:else}
			<form
				id={telegramConnectFormId}
				method="POST"
				action="?/telegram"
				use:enhance
				class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4"
				aria-labelledby="telegram-connect"
			>
				<input type="hidden" name="workspaceId" value={$formData.workspaceId} />
				<input type="hidden" name="agentId" value={$formData.agentId} />

				<div class="flex flex-col gap-1">
					<h2 id="telegram-connect" class="text-md font-medium tracking-snug text-ink-900">
						{bot ? "Replace the token" : "Connect a bot"}
					</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{bot
							? "Paste a new token for the same bot to keep everyone linked. A different bot starts with nobody linked."
							: "In Telegram, message @BotFather, send /newbot, and paste the token it gives you. Norn points the bot at itself, so the bot stops answering anything else it was connected to."}
					</p>
				</div>

				<Form.Field {form} name="token">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Bot token</Form.Label>
							<Input
								{...props}
								type="password"
								autocomplete="off"
								autocapitalize="none"
								spellcheck="false"
								placeholder={bot?.tokenHint ? `Stored, ending ${bot.tokenHint}` : "123456789:AA…"}
								disabled={busy}
								bind:value={$formData.token}
							/>
						{/snippet}
					</Form.Control>
					<Form.Description class="text-sm text-muted-foreground">
						It is proved against Telegram before it is stored, and it is stored encrypted.
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				<div>
					<Button type="submit" disabled={busy}>
						{$submitting ? "Connecting…" : bot ? "Replace token" : "Connect bot"}
					</Button>
				</div>
			</form>
		{/if}
	{/if}
</div>

<AlertDialog.Root
	open={confirmingDisconnect}
	onOpenChange={(open) => {
		if (!open && mutation === null) confirmingDisconnect = false;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Disconnect {bot ? telegramHandle(bot.username) : "the bot"}?</AlertDialog.Title>
			<AlertDialog.Description>
				{agentName} stops posting to Telegram, and everyone and every group linked to the bot is forgotten.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			{#if mutation !== null}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action variant="destructive" disabled={mutation !== null} onclick={disconnect}>
				{mutation === "disconnect" ? "Disconnecting…" : "Disconnect bot"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
