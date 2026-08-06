<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import Bot from "@lucide/svelte/icons/bot";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { onDate } from "$lib/time";
	import { registerAgentSchema } from "$lib/agents/register-agent-schema";
	import {
		agentPath,
		agentScopeGroups,
		agentScopeLabels,
		approvalsPath,
		failureMessage,
		type AgentFailure,
		type AgentListing,
	} from "$lib/agents/agents";
	import { agentsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "register-agent-form";
	const defaultLimit = 120;

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? agentsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let copiedValue = $state<string | null>(null);
	let disabling = $state<string | null>(null);
	let disableFailure = $state<AgentFailure | null>(null);

	const workspace = $derived(data.workspace);
	const listing = $derived<AgentListing>(preview?.listing ?? data.listing);
	const people = $derived(preview?.people ?? data.people);
	const teams = $derived(preview?.teams ?? data.teams);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		validators: zod4Client(registerAgentSchema),
		resetForm: false,
		onSubmit: () => {
			disableFailure = null;
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	const outcome = $derived($message ?? null);
	const issued = $derived(outcome && outcome.kind === "issued" ? outcome : null);
	const failure = $derived<AgentFailure | null>(
		disableFailure ?? (outcome && outcome.kind !== "issued" ? outcome : null)
	);

	$effect(() => {
		if ($formData.actionLimit === 0) {
			formData.update((entered) => ({
				...entered,
				actionLimit: defaultLimit,
				allTeams: true,
				ownerAccountId: entered.ownerAccountId || data.member.id,
			}));
		}
	});

	const busy = $derived(preview?.busy || $submitting || disabling !== null);

	const current = $derived(
		listing.kind === "ready" || listing.kind === "registered" ? listing.agents : []
	);

	const registered = $derived(listing.kind === "registered" ? listing : issued);
	const copied = $derived(copiedValue === registered?.value);
	const showForm = $derived(listing.kind !== "forbidden" && listing.kind !== "unavailable");

	const ownerName = $derived(
		people.find((person) => person.accountId === $formData.ownerAccountId)?.displayName ??
			"Choose a person"
	);

	function toggleScope(scope: string, checked: boolean) {
		const next = new Set($formData.scopes);

		if (checked) {
			next.add(scope);
		} else {
			next.delete(scope);
		}

		formData.update((entered) => ({ ...entered, scopes: [...next] }));
	}

	function toggleTeam(teamId: string, checked: boolean) {
		const next = new Set($formData.teamIds);

		if (checked) {
			next.add(teamId);
		} else {
			next.delete(teamId);
		}

		formData.update((entered) => ({ ...entered, teamIds: [...next] }));
	}

	async function copyValue() {
		if (!registered) return;
		await navigator.clipboard.writeText(registered.value);
		copiedValue = registered.value;
	}

	async function disable(agentId: string) {
		disabling = agentId;
		disableFailure = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/agents/{agentId}", {
				params: { path: { workspaceId: workspace.id, agentId } },
			});

			if (error) {
				disableFailure = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			await invalidate(keys.page(page.route.id));
		} catch {
			disableFailure = { kind: "unavailable" };
		} finally {
			disabling = null;
		}
	}

	function formatted(instant: string | undefined): string | null {
		if (!instant) return null;

		return onDate(instant, workspace.timezone);
	}
</script>

<svelte:head><title>Agents · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center justify-between gap-3 border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Agents</h1>
		<a
			href={approvalsPath(workspace.slug)}
			class="text-xs text-muted-foreground transition-colors hover:text-ink-900"
		>
			Waiting for approval
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if listing.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage agents here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the agents</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if registered}
				<section class="flex flex-col gap-3 rounded-lg border border-success/40 bg-paper-0 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							Copy {registered.agent.name}'s credential now
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							This is the only time it is shown. If you lose it, disable the agent and register
							another.
						</p>
					</div>
					<p class="rounded-md bg-paper-100 p-3 font-mono text-xs break-all text-ink-900">
						{registered.value}
					</p>
					<div>
						<Button variant="secondary" size="sm" onclick={copyValue}>
							{copied ? "Copied" : "Copy credential"}
						</Button>
					</div>
				</section>
			{/if}

			{#if showForm}
				<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
					<input type="hidden" name="workspaceId" value={workspace.id} />
					{#if $formData.allTeams}
						<input type="hidden" name="allTeams" value="true" />
					{/if}
					{#each $formData.scopes as scope (scope)}
						<input type="hidden" name="scopes" value={scope} />
					{/each}
					{#each $formData.teamIds as teamId (teamId)}
						<input type="hidden" name="teamIds" value={teamId} />
					{/each}
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Register an agent</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							An agent acts under a person's authority and can never do more than they can. If
							their access changes, the agent's changes with it.
						</p>
					</div>

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

					<Form.Field {form} name="ownerAccountId">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Acts for</Form.Label>
								<Select.Root
									type="single"
									name={props.name}
									bind:value={$formData.ownerAccountId}
									disabled={busy}
								>
									<Select.Trigger {...props}>{ownerName}</Select.Trigger>
									<Select.Content>
										{#each people as person (person.accountId)}
											<Select.Item value={person.accountId}>
												{person.displayName || person.email}
											</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
								<Form.Description>
									Everything the agent may do is bounded by this person.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="scopes">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Permissions</Form.Label>
								<div {...props} class="flex flex-col gap-3">
									{#each agentScopeGroups as group (group.title)}
										<div class="flex flex-col gap-1.5">
											<p class="text-xs font-medium tracking-snug text-ink-900">{group.title}</p>
											{#each group.scopes as scope (scope)}
												<div class="flex items-start gap-2">
													<Checkbox
														id={`agent-scope-${scope}`}
														disabled={busy}
														checked={$formData.scopes.includes(scope)}
														onCheckedChange={(checked) => toggleScope(scope, checked === true)}
													/>
													<label
														for={`agent-scope-${scope}`}
														class="text-sm leading-normal text-pretty text-ink-600"
													>
														{agentScopeLabels[scope] ?? scope}
														<span class="font-mono text-xs text-muted-foreground">{scope}</span>
													</label>
												</div>
											{/each}
										</div>
									{/each}
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="allTeams">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Where it reaches</Form.Label>
								<div {...props} class="flex flex-col gap-2">
									<div class="flex items-center gap-2">
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
										<label for="agent-all-teams" class="text-sm leading-normal text-ink-600">
											Every team its owner can see
										</label>
									</div>

									{#if !$formData.allTeams}
										{#if teams.length === 0}
											<p class="text-xs text-muted-foreground">No teams you can see here.</p>
										{:else}
											<div class="flex flex-col gap-1.5 pl-6">
												{#each teams as team (team.id)}
													<div class="flex items-center gap-2">
														<Checkbox
															id={`agent-team-${team.id}`}
															disabled={busy}
															checked={$formData.teamIds.includes(team.id)}
															onCheckedChange={(checked) => toggleTeam(team.id, checked === true)}
														/>
														<label
															for={`agent-team-${team.id}`}
															class="text-sm leading-normal text-ink-600"
														>
															{team.name}
														</label>
													</div>
												{/each}
											</div>
										{/if}
									{/if}
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

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
								<Form.Description>
									Changes only. An agent may read as much as it likes.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<div>
						<Form.Button disabled={busy}>
							{$submitting ? "Registering…" : "Register agent"}
						</Form.Button>
					</div>
				</form>
			{/if}

			{#if listing.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading agents…</p>
			{:else if current.length === 0 && showForm}
				<p class="text-sm text-muted-foreground">No agents act in {workspace.name} yet.</p>
			{:else if current.length > 0}
				<section class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Registered agents</h2>
					<ul class="rounded-lg border border-line-subtle bg-paper-0">
						{#each current as owned (owned.agent.id)}
							<li
								class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
							>
								<div class="flex min-w-0 flex-col gap-1.5">
									<div class="flex flex-wrap items-center gap-2">
										<Bot class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
										<a
											href={agentPath(workspace.slug, owned.agent.id)}
											class="truncate text-sm text-ink-900 underline-offset-2 hover:underline"
										>
											{owned.agent.name}
										</a>
										<Tag name="Agent" />
										{#if owned.agent.status === "disabled"}
											<Tag name="Disabled" />
										{/if}
									</div>
									<p class="text-xs text-muted-foreground">
										Acts for {owned.ownerName || owned.ownerEmail || "somebody who has left"}
									</p>
									<p class="text-xs text-muted-foreground">
										Registered {formatted(owned.agent.createdAt)}
										· {owned.agent.actionLimit} actions a minute
									</p>
								</div>
								<div class="flex-none">
									{#if owned.agent.status === "active"}
										<Button
											variant="secondary"
											size="sm"
											disabled={busy}
											onclick={() => disable(owned.agent.id)}
										>
											{disabling === owned.agent.id ? "Disabling…" : "Disable"}
										</Button>
									{:else}
										<span class="text-xs text-muted-foreground">Stopped</span>
									{/if}
								</div>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		</div>
	</div>
</div>
