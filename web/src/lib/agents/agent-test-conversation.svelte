<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Wrench from "@lucide/svelte/icons/wrench";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { api } from "$lib/api";
	import { aiProviderPath } from "$lib/ai-provider/ai-provider";
	import {
		conversationFailure,
		conversationFailureMessage,
		conversationFailureTitle,
		conversationFull,
		conversationTurns,
		stopNote,
		tokenCount,
		transientFailures,
		type HostedConversation,
	} from "./hosted";
	import { hostedQuestionSchema } from "./hosted-question-schema";

	let {
		workspace,
		agentId,
		administrator,
		selfHosted,
		opened,
	}: {
		workspace: { id: string; slug: string };
		agentId: string;
		administrator: boolean;
		selfHosted: boolean;
		opened?: HostedConversation;
	} = $props();

	// svelte-ignore state_referenced_locally
	let conversation = $state<HostedConversation>(opened ?? { kind: "idle", exchanges: [] });

	const tokens = new Intl.NumberFormat("en");

	const form = superForm(defaults({ question: "" }, zod4(hostedQuestionSchema)), {
		id: "hosted-question-form",
		SPA: true,
		validators: zod4Client(hostedQuestionSchema),
		resetForm: false,
		invalidateAll: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			if (await ask(pending.data.question)) pending.data.question = "";
		},
	});

	const { form: formData, enhance, submitting } = form;

	const asking = $derived(conversation.kind === "asking" || $submitting);
	const full = $derived(conversationFull(conversation.exchanges));
	const fixable = $derived(
		conversation.kind === "failed" &&
			administrator &&
			(conversation.failure.kind === "not_configured" ||
				(conversation.failure.kind === "provider" && !transientFailures.includes(conversation.failure.code)))
	);

	async function ask(question: string): Promise<boolean> {
		const exchanges = conversation.exchanges;

		conversation = { kind: "asking", exchanges, question };

		try {
			const { data: reply, error, response } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/conversation",
				{
					params: { path: { workspaceId: workspace.id, agentId } },
					body: { turns: conversationTurns(exchanges, question) },
				}
			);

			if (error || !reply) {
				conversation = {
					kind: "failed",
					exchanges,
					question,
					failure: conversationFailure(error, response.status),
				};

				return false;
			}

			conversation = { kind: "idle", exchanges: [...exchanges, { question, reply }] };

			return true;
		} catch {
			conversation = { kind: "failed", exchanges, question, failure: { kind: "unavailable" } };

			return false;
		}
	}

	async function retry() {
		if (conversation.kind !== "failed") return;

		if (await ask(conversation.question)) form.reset();
	}

	function clear() {
		conversation = { kind: "idle", exchanges: [] };
		form.reset();
	}
</script>

<section class="flex flex-col gap-4 rounded-lg border border-line-strong p-4" aria-labelledby="hosted-test-title">
	<div>
		<h2 id="hosted-test-title" class="text-md font-medium tracking-snug text-ink-900">Test this agent</h2>
		<p class="mt-1 text-sm leading-normal text-muted-foreground text-pretty">
			Ask it about this workspace. It answers as itself, with the permissions on its overview, and
			nothing here is kept once you leave the page.
		</p>
	</div>

	{#if conversation.exchanges.length > 0 || conversation.kind !== "idle"}
		<ol class="flex flex-col gap-4" aria-label="Conversation">
			{#each conversation.exchanges as exchange, index (index)}
				<li class="flex flex-col gap-2">
					<p class="self-end rounded-md bg-paper-1 px-3 py-2 text-sm leading-normal whitespace-pre-wrap text-ink-900">
						{exchange.question}
					</p>
					<div class="flex flex-col gap-2 border-l border-line-subtle pl-3">
						{#if exchange.reply.text}
							<p class="text-sm leading-normal whitespace-pre-wrap text-ink-900">{exchange.reply.text}</p>
						{/if}
						{#if stopNote(exchange.reply.stop)}
							<p class="text-xs leading-normal text-muted-foreground">{stopNote(exchange.reply.stop)}</p>
						{/if}
						{#if exchange.reply.toolCalls.length > 0}
							<ul class="flex flex-col gap-1" aria-label="Tools used">
								{#each exchange.reply.toolCalls as call, callIndex (callIndex)}
									<li class="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
										<Wrench class="size-3" aria-hidden="true" />
										<code class="font-mono">{call.name}</code>
										{#if call.refusal}
											<span>refused · <code class="font-mono">{call.refusal}</code></span>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
						<p class="text-xs text-ink-400">{tokens.format(tokenCount(exchange.reply))} tokens</p>
					</div>
				</li>
			{/each}

			{#if conversation.kind !== "idle"}
				<li class="flex flex-col gap-2">
					<p class="self-end rounded-md bg-paper-1 px-3 py-2 text-sm leading-normal whitespace-pre-wrap text-ink-900">
						{conversation.question}
					</p>
					{#if conversation.kind === "asking"}
						<div class="flex flex-col gap-2 border-l border-line-subtle pl-3" role="status" aria-busy="true">
							<span class="sr-only">The agent is working on an answer.</span>
							<Skeleton class="h-3 w-full max-w-96" />
							<Skeleton class="h-3 w-64" />
						</div>
					{/if}
				</li>
			{/if}
		</ol>
	{/if}

	{#if conversation.kind === "failed"}
		<Alert.Root variant={conversation.failure.kind === "not_configured" ? "warning" : "destructive"}>
			<CircleAlert aria-hidden="true" />
			<Alert.Title>{conversationFailureTitle(conversation.failure)}</Alert.Title>
			<Alert.Description>
				{conversationFailureMessage(conversation.failure, administrator, selfHosted)}
			</Alert.Description>
			<Alert.Action placement="below">
				{#if fixable}
					<Button href={aiProviderPath(workspace.slug)} variant="secondary" size="sm">
						AI provider settings
					</Button>
				{:else}
					<Button variant="secondary" size="sm" onclick={retry}>Ask again</Button>
				{/if}
			</Alert.Action>
		</Alert.Root>
	{/if}

	{#if full}
		<Alert.Root variant="muted">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>This conversation is as long as a test may be</Alert.Title>
			<Alert.Description>Clear it to start again.</Alert.Description>
		</Alert.Root>
	{/if}

	<form id="hosted-question-form" method="POST" use:enhance class="flex flex-col gap-3">
		<Form.Field {form} name="question">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Question</Form.Label>
					<Textarea
						{...props}
						rows={3}
						bind:value={$formData.question}
						disabled={asking || full}
						placeholder="Which issues in progress have no assignee?"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<div class="flex flex-wrap justify-end gap-2">
			<Button
				type="button"
				variant="secondary"
				size="sm"
				disabled={asking || conversation.exchanges.length === 0}
				onclick={clear}
			>
				Clear
			</Button>
			<Button type="submit" size="sm" disabled={asking || full}>
				{asking ? "Asking…" : "Ask"}
			</Button>
		</div>
	</form>
</section>
