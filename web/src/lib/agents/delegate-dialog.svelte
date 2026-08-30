<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { page } from "$app/state";
	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { runnersPath } from "$lib/runners/runners";
	import { agentsPath } from "./agents";
	import { delegateIssueSchema } from "./delegate-schema";
	import {
		delegationFailureMessage,
		readDelegationFailure,
		type DelegationFailure,
		type IssueDelegation,
		type Member,
	} from "./delegation";

	let {
		open = $bindable(false),
		workspaceId,
		issueId,
		reference,
		agents,
		ondelegated,
	}: {
		open?: boolean;
		workspaceId: string;
		issueId: string;
		reference: string;
		agents: Member[];
		ondelegated: (delegation: IssueDelegation) => void;
	} = $props();

	const formId = "delegate-issue-form";

	let failure = $state<DelegationFailure | null>(null);

	const form = superForm(defaults(zod4(delegateIssueSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(delegateIssueSchema),
		resetForm: true,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;

			try {
				const { data, error } = await api.POST(
					"/workspaces/{workspaceId}/issues/{issueId}/delegation",
					{
						params: { path: { workspaceId, issueId } },
						body: {
							agentAccountId: entered.data.agentAccountId,
							brief: entered.data.brief || undefined,
						},
					}
				);

				if (error) {
					failure = readDelegationFailure(error);

					return;
				}

				if (!data) {
					failure = { kind: "unavailable" };

					return;
				}

				ondelegated(data);
				open = false;
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const chosen = $derived(agents.find((agent) => agent.accountId === $formData.agentAccountId));
	const mcpOnly = $derived(chosen !== undefined && !chosen.hasRunner);

	function nameOf(agent: Member): string {
		return agent.displayName || agent.email || "An agent";
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-120">
		<Dialog.Header>
			<Dialog.Title>Hand {reference} to an agent</Dialog.Title>
			<Dialog.Description>
				Norn does not run anything itself. Delegating announces the work so whatever runs your
				agents can pick it up, and records who is holding it.
			</Dialog.Description>
		</Dialog.Header>

		{#if agents.length === 0}
			<p class="text-md leading-normal text-muted-foreground text-pretty">
				You have not registered an agent yet. Norn only hands work to agents you registered, so
				this stays empty until one of yours exists.
			</p>
			<a
				href={agentsPath(page.params.workspace ?? "")}
				class="w-fit text-sm text-link underline-offset-2 hover:underline"
			>
				Register an agent
			</a>
			<Dialog.Footer>
				<Button variant="secondary" onclick={() => (open = false)}>Close</Button>
			</Dialog.Footer>
		{:else}
			<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
				<Form.Field {form} name="agentAccountId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Agent</Form.Label>
							<Select.Root
								type="single"
								value={$formData.agentAccountId}
								disabled={$submitting}
								onValueChange={(value) => ($formData.agentAccountId = value)}
							>
								<Select.Trigger {...props}>
									{chosen ? nameOf(chosen) : "Choose an agent"}
								</Select.Trigger>
								<Select.Content>
									{#each agents as agent (agent.accountId)}
										<Select.Item value={agent.accountId} label={nameOf(agent)}>
											<Bot class="size-3.5 text-muted-foreground" aria-hidden="true" />
											{nameOf(agent)}
											{#if !agent.hasRunner}
												<Tag name="MCP only" class="ml-auto" />
											{/if}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				{#if mcpOnly && chosen}
					<Alert.Root variant="muted">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>{nameOf(chosen)} has no machine connected</Alert.Title>
						<Alert.Description>
							It can still read and change this issue through an AI client, but there is nowhere
							for it to run code, so the work waits until a computer is connected to it. You can
							hand it over anyway — it starts as soon as one is.
						</Alert.Description>
						<Alert.Action>
							<a
								href={runnersPath(page.params.workspace ?? "")}
								class="text-sm text-link underline-offset-2 hover:underline"
							>
								Connect a machine
							</a>
						</Alert.Action>
					</Alert.Root>
				{/if}

				<Form.Field {form} name="brief">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>What it is being asked to do</Form.Label>
							<Textarea
								{...props}
								bind:value={$formData.brief}
								disabled={$submitting}
								rows={4}
								placeholder="Reproduce the retry bug, fix it, and prove it with the failing test first."
							/>
						{/snippet}
					</Form.Control>
					<Form.Description class="text-sm text-muted-foreground">
						Optional. This travels with the event, so the runtime that picks the work up reads it.
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				{#if failure}
					<p class="text-sm leading-normal text-destructive text-pretty" role="alert">
						{delegationFailureMessage(failure)}
					</p>
				{/if}

				<Dialog.Footer>
					<Button
						type="button"
						variant="secondary"
						disabled={$submitting}
						onclick={() => (open = false)}
					>
						Cancel
					</Button>
					<Form.Button disabled={$submitting}>
						{$submitting ? "Handing it over…" : "Delegate"}
					</Form.Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
