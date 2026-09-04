<script lang="ts">
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import {
		defaults,
		superForm,
		type Infer,
		type SuperValidated,
	} from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import type { Team } from "$lib/team/teams";
	import { cn } from "$lib/utils.js";
	import AgentIcon from "./agent-icon.svelte";
	import CredentialDialog from "./credential-dialog.svelte";
	import {
		agentIconLabels,
		agentIcons,
		agentScopeGroups,
		agentScopeLabels,
		agentScopes,
		beyondGrant,
		failureMessage,
		type Agent,
		type AgentFailure,
		type AgentScope,
		type APIScope,
	} from "./agents";
	import { defaultAgentActionLimit, registerAgentSchema } from "./register-agent-schema";

	type RegisterForm = Infer<typeof registerAgentSchema>;
	type RegistrationOutcome = { kind: "issued"; agent: Agent; value: string } | AgentFailure;

	const formId = "register-agent-form";
	const withheldId = "agent-scopes-withheld";

	let {
		open,
		workspaceId,
		workspaceName,
		origin,
		teams,
		grantable = null,
		initial,
		inline = false,
		closeHref,
		disabled = false,
		onclose,
		onissued,
	}: {
		open: boolean;
		workspaceId: string;
		workspaceName: string;
		origin: string;
		teams: Team[];
		grantable?: APIScope[] | null;
		initial?: SuperValidated<RegisterForm, RegistrationOutcome>;
		inline?: boolean;
		closeHref?: string;
		disabled?: boolean;
		onclose: () => void;
		onissued: (agent: Agent, value: string) => void | Promise<void>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const form = superForm(initial ?? defaults(zod4(registerAgentSchema)), {
		id: formId,
		validators: zod4Client(registerAgentSchema),
		resetForm: false,
		invalidateAll: false,
	});
	const { form: formData, enhance, submitting, message } = form;

	let wasOpen = $state(false);
	let deliveredAgentId = $state("");

	const busy = $derived(disabled || $submitting);
	const outcome = $derived<RegistrationOutcome | null>($message ?? null);
	const refused = $derived(outcome && outcome.kind !== "issued" ? outcome : null);
	const failure = $derived<AgentFailure | null>(
		refused?.kind === "scope_exceeds"
			? { kind: "scope_exceeds", scopes: beyondGrant($formData.scopes, grantable) }
			: refused
	);
	const withheld = $derived(new Set(beyondGrant([...agentScopes], grantable)));

	$effect(() => {
		if (!inline && open && !wasOpen) {
			form.reset({ keepMessage: false });
			formData.update(
				(entered) => ({
					...entered,
					icon: "bot",
					scopes: [],
					allTeams: true,
					teamIds: [],
					actionLimit: defaultAgentActionLimit,
				}),
				{ taint: false }
			);
		}

		wasOpen = open;
	});

	$effect(() => {
		const issued = outcome?.kind === "issued" ? outcome : null;

		if (inline || !issued || issued.agent.id === deliveredAgentId) return;

		deliveredAgentId = issued.agent.id;
		const pending = onissued(issued.agent, issued.value);
		message.set(undefined);
		form.reset({ keepMessage: false });
		onclose();
		void pending;
	});

	function toggleScope(scope: AgentScope, checked: boolean) {
		const next = new Set($formData.scopes);

		if (checked) next.add(scope);
		else next.delete(scope);

		formData.update((entered) => ({ ...entered, scopes: [...next] }));
	}

	function toggleTeam(teamId: string, checked: boolean) {
		const next = new Set($formData.teamIds);

		if (checked) next.add(teamId);
		else next.delete(teamId);

		formData.update((entered) => ({ ...entered, teamIds: [...next] }));
	}

	function changeOpen(next: boolean) {
		if (!next && !busy) onclose();
	}
</script>

{#snippet registration()}
	{#if failure}
		<Alert.Root variant="destructive">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>Agent not registered</Alert.Title>
			<Alert.Description>{failureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
		<input type="hidden" name="workspaceId" value={workspaceId} />
		<input type="hidden" name="allTeams" value="false" />
		{#if !inline}
			{#if $formData.allTeams}
				<input type="hidden" name="allTeams" value="true" />
			{/if}
			{#each $formData.scopes as scope (scope)}
				<input type="hidden" name="scopes" value={scope} />
			{/each}
			{#each $formData.teamIds as teamId (teamId)}
				<input type="hidden" name="teamIds" value={teamId} />
			{/each}
		{/if}

		<Form.Field {form} name="name">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Name</Form.Label>
					<Input
						{...props}
						bind:value={$formData.name}
						disabled={busy}
						placeholder="triage-bot"
						autocomplete="off"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="icon">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Icon</Form.Label>
					{#if inline}
						<div {...props} class="grid grid-cols-2 gap-2 sm:grid-cols-5">
							{#each agentIcons as icon (icon)}
								<label
									for={`agent-icon-${icon}`}
									class={cn(
										"flex min-w-0 cursor-pointer items-center gap-2 rounded-md border border-line-default bg-paper-1 px-2 py-2 text-xs text-ink-600 motion-control hover:border-ink-400 hover:bg-paper-2",
										$formData.icon === icon && "border-ink-900 text-ink-900 rule-inset"
									)}
								>
									<input
										id={`agent-icon-${icon}`}
										type="radio"
										name={props.name}
										value={icon}
										checked={$formData.icon === icon}
										disabled={busy}
										onchange={() => ($formData.icon = icon)}
										class="size-3.5 accent-ink-900"
									/>
									<AgentIcon {icon} class="size-4 shrink-0" />
									<span class="truncate">{agentIconLabels[icon]}</span>
								</label>
							{/each}
						</div>
					{:else}
						<RadioGroup.Root
							{...props}
							bind:value={$formData.icon}
							class="grid-cols-2 sm:grid-cols-5"
							disabled={busy}
						>
							{#each agentIcons as icon (icon)}
								<label
									for={`agent-icon-${icon}`}
									class={cn(
										"flex min-w-0 cursor-pointer items-center gap-2 rounded-md border border-line-default bg-paper-1 px-2 py-2 text-xs text-ink-600 motion-control hover:border-ink-400 hover:bg-paper-2",
										$formData.icon === icon && "border-ink-900 text-ink-900 rule-inset"
									)}
								>
									<RadioGroup.Item id={`agent-icon-${icon}`} value={icon} />
									<AgentIcon {icon} class="size-4 shrink-0" />
									<span class="truncate">{agentIconLabels[icon]}</span>
								</label>
							{/each}
						</RadioGroup.Root>
					{/if}
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Fieldset {form} name="scopes">
			<Form.Legend>Permissions</Form.Legend>
			<div class="grid gap-3 sm:grid-cols-2">
				{#each agentScopeGroups as group (group.title)}
					<fieldset class="flex min-w-0 flex-col gap-1.5">
						<legend class="mb-2 text-xs font-medium tracking-snug text-ink-900">
							{group.title}
						</legend>
						{#each group.scopes as scope (scope)}
							<div class="flex items-start gap-2">
								{#if inline}
									<input
										id={`agent-scope-${scope}`}
										type="checkbox"
										name="scopes"
										value={scope}
										disabled={busy || withheld.has(scope)}
										checked={$formData.scopes.includes(scope)}
										onchange={(event) => toggleScope(scope, event.currentTarget.checked)}
										aria-describedby={withheld.has(scope) ? withheldId : undefined}
										class="mt-0.5 size-3.5 accent-ink-900"
									/>
								{:else}
									<Checkbox
										id={`agent-scope-${scope}`}
										disabled={busy || withheld.has(scope)}
										checked={$formData.scopes.includes(scope)}
										onCheckedChange={(checked) => toggleScope(scope, checked === true)}
										aria-describedby={withheld.has(scope) ? withheldId : undefined}
									/>
								{/if}
								<label
									for={`agent-scope-${scope}`}
									class={cn(
										"text-xs leading-normal text-ink-600",
										withheld.has(scope) && "text-ink-400"
									)}
								>
									{agentScopeLabels[scope] ?? scope}
								</label>
							</div>
						{/each}
					</fieldset>
				{/each}
			</div>
			{#if withheld.size > 0}
				<p id={withheldId} class="text-xs leading-normal text-ink-400">
					An agent cannot do more than you can, so the greyed permissions need an administrator.
				</p>
			{/if}
			<Form.FieldErrors />
		</Form.Fieldset>

		<Form.Fieldset {form} name="allTeams">
			<Form.Legend>Team access</Form.Legend>
			<div class="flex flex-col gap-2">
				<div class="flex items-center gap-2">
					{#if inline}
						<input
							id="agent-all-teams"
							type="checkbox"
							name="allTeams"
							value="true"
							disabled={busy}
							checked={$formData.allTeams}
							onchange={(event) =>
								formData.update((entered) => ({
									...entered,
									allTeams: event.currentTarget.checked,
									teamIds: [],
								}))}
							class="size-3.5 accent-ink-900"
						/>
					{:else}
						<Checkbox
							id="agent-all-teams"
							disabled={busy}
							checked={$formData.allTeams}
							onCheckedChange={(checked) =>
								formData.update((entered) => ({
									...entered,
									allTeams: checked === true,
									teamIds: [],
								}))}
						/>
					{/if}
					<label for="agent-all-teams" class="text-sm leading-normal text-ink-600">
						Every team you can see
					</label>
				</div>

				{#if inline || !$formData.allTeams}
					{#if teams.length === 0}
						<p class="text-xs text-muted-foreground">No teams are available.</p>
					{:else}
						<div class="grid gap-1.5 pl-6 sm:grid-cols-2">
							{#each teams as team (team.id)}
								<div class="flex items-center gap-2">
									{#if inline}
										<input
											id={`agent-team-${team.id}`}
											type="checkbox"
											name="teamIds"
											value={team.id}
											disabled={busy}
											checked={$formData.teamIds.includes(team.id)}
											onchange={(event) => toggleTeam(team.id, event.currentTarget.checked)}
											class="size-3.5 accent-ink-900"
										/>
									{:else}
										<Checkbox
											id={`agent-team-${team.id}`}
											disabled={busy}
											checked={$formData.teamIds.includes(team.id)}
											onCheckedChange={(checked) => toggleTeam(team.id, checked === true)}
										/>
									{/if}
									<label for={`agent-team-${team.id}`} class="text-sm text-ink-600">
										{team.name}
									</label>
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>
			<Form.FieldErrors />
		</Form.Fieldset>

		<Form.Field {form} name="actionLimit">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Actions a minute</Form.Label>
					<Input
						{...props}
						type="number"
						min="1"
						max="6000"
						disabled={busy}
						bind:value={$formData.actionLimit}
					/>
					<Form.Description>Changes only. Reads are not limited.</Form.Description>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		{#if inline}
			<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
				<Button href={closeHref} variant="ghost" disabled={busy} onclick={onclose}>Cancel</Button>
				<Form.Button disabled={busy}>
					{$submitting ? "Registering…" : "Register agent"}
				</Form.Button>
			</div>
		{:else}
			<Dialog.Footer>
				<Button variant="ghost" disabled={busy} onclick={onclose}>Cancel</Button>
				<Form.Button disabled={busy}>
					{$submitting ? "Registering…" : "Register agent"}
				</Form.Button>
			</Dialog.Footer>
		{/if}
	</form>
{/snippet}

{#if inline && outcome?.kind === "issued"}
	<CredentialDialog
		open
		agent={outcome.agent}
		value={outcome.value}
		{origin}
		{workspaceName}
		inline
		doneHref={closeHref}
		onclose={() => message.set(undefined)}
	/>
{:else if inline}
	<section class="notch flex flex-col gap-6 p-6" aria-labelledby="register-agent-title">
		<header class="flex flex-col gap-1.5">
			<h2 id="register-agent-title" class="text-lg font-medium tracking-snug text-ink-900">
				Register an agent
			</h2>
			<p class="text-sm leading-normal text-muted-foreground">
				It will act as you. Give it only the authority it needs within this workspace.
			</p>
		</header>
		{@render registration()}
	</section>
{:else}
	<Dialog.Root {open} onOpenChange={changeOpen}>
		<Dialog.Content
			variant="scrollable"
			class="sm:max-w-168"
			showCloseButton={!busy}
			onEscapeKeydown={(event) => busy && event.preventDefault()}
			onInteractOutside={(event) => busy && event.preventDefault()}
		>
			<Dialog.Header>
				<Dialog.Title>Register an agent</Dialog.Title>
				<Dialog.Description>
					It will act as you. Give it only the authority it needs within this workspace.
				</Dialog.Description>
			</Dialog.Header>
			{@render registration()}
		</Dialog.Content>
	</Dialog.Root>
{/if}
