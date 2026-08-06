<script lang="ts">
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Lock from "@lucide/svelte/icons/lock";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { createTeamSchema } from "$lib/team/create-team-schema";
	import {
		teamKeyFromName,
		teamListTabs,
		teamsIn,
		visibilityLabels,
		visibilityNotes,
		type Team,
		type TeamCreationFailure,
		type TeamListTab,
		type TeamListing,
		type TeamVisibility,
	} from "$lib/team/teams";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "create-team-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? teamsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const slug = $derived(page.params.workspace ?? "");

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		validators: zod4Client(createTeamSchema),
		resetForm: false,
	});
	const { form: formData, enhance, submitting, message } = form;

	const loaded = $derived<TeamListing>(preview?.listing ?? data.listing);
	const outcome = $derived($message ?? null);

	const listing = $derived<TeamListing>(
		outcome?.kind === "created" && "teams" in loaded
			? { kind: "created", teams: loaded.teams, team: outcome.team }
			: loaded
	);
	const failure = $derived<TeamCreationFailure | null>(
		preview?.failure ?? (outcome && outcome.kind !== "created" ? outcome : null)
	);

	const teams = $derived("teams" in listing ? listing.teams : []);
	const tab = $derived<TeamListTab>(
		page.url.searchParams.get("tab") === "archived" ? "archived" : "active"
	);
	const shown = $derived(teamsIn(teams, tab));

	const creating = $derived(
		listing.kind === "empty" || preview?.view === "create" || page.url.searchParams.has("new")
	);

	let keyEdited = $state(false);

	$effect(() => {
		const prefill = preview?.form;
		if (prefill) formData.update((current) => ({ ...current, ...prefill }), { taint: false });
	});

	const busy = $derived(preview?.busy || $submitting);
	const keyTaken = $derived(
		failure?.kind === "key_taken" && failure.key === $formData.key.toUpperCase()
	);
	const visibilities: TeamVisibility[] = ["public", "private"];

	function tabHref(candidate: TeamListTab): string {
		return workspacePath(slug, `/settings/teams${candidate === "archived" ? "?tab=archived" : ""}`);
	}

	function teamHref(team: Team): string {
		return workspacePath(slug, `/settings/teams/${team.key}`);
	}

	function keyFieldValue(event: Event) {
		return (event.currentTarget as HTMLInputElement).value;
	}

	function deriveKey(event: Event) {
		if (keyEdited) return;
		$formData.key = teamKeyFromName(keyFieldValue(event));
	}

	function claimKey(event: Event) {
		keyEdited = keyFieldValue(event).trim() !== "";
	}

	function pickKey(key: string) {
		keyEdited = true;
		$formData.key = key;
	}
</script>

<svelte:head><title>Teams · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Users class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Teams</h1>
			<div class="flex-1"></div>
			{#if !creating}
				<Button size="sm" href={workspacePath(slug, "/settings/teams?new")}>New team</Button>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if listing.kind === "created"}
				<Alert.Root variant="success">
					<CircleCheck aria-hidden="true" />
					<Alert.Title>{listing.team.name} is ready</Alert.Title>
					<Alert.Description>
						Its issues will be numbered {listing.team.key}-1, {listing.team.key}-2, and so on.
					</Alert.Description>
					<Alert.Action>
						<Button variant="secondary" size="sm" href={teamHref(listing.team)}>
							Open {listing.team.name}
						</Button>
					</Alert.Action>
				</Alert.Root>
			{/if}

			{#if listing.kind === "unavailable" || failure?.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load the teams</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if failure?.kind === "forbidden"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>Only admins can create teams</Alert.Title>
					<Alert.Description>
						Ask a workspace administrator to create it for you.
					</Alert.Description>
				</Alert.Root>
			{/if}

			{#if creating}
				<section class="flex flex-col gap-4 rounded-lg border border-line-default p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Create a team</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Teams own issues. The key is stamped on every issue the team raises, and it is
							permanent.
						</p>
					</div>

					<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
						<input type="hidden" name="workspaceId" value={data.workspace.id} />
						<div class="flex flex-wrap gap-3">
							<div class="min-w-[150px] flex-[1_1_180px]">
								<Form.Field {form} name="name">
									<Form.Control>
										{#snippet children({ props })}
											<Form.Label>Team name</Form.Label>
											<Input
											{...props}
											disabled={busy}
											oninput={deriveKey}
											bind:value={$formData.name}
										/>
										{/snippet}
									</Form.Control>
									<Form.FieldErrors />
								</Form.Field>
							</div>
							<div class="min-w-20 flex-[0_1_92px]">
								<Form.Field {form} name="key">
									<Form.Control>
										{#snippet children({ props })}
											<Form.Label>Key</Form.Label>
											<Input
												{...props}
												maxlength={5}
												autocapitalize="characters"
												spellcheck="false"
												disabled={busy}
												aria-invalid={keyTaken ? "true" : undefined}
												class="uppercase"
												oninput={claimKey}
												bind:value={$formData.key}
											/>
										{/snippet}
									</Form.Control>
									<Form.FieldErrors />
								</Form.Field>
							</div>
						</div>

						{#if keyTaken && failure?.kind === "key_taken"}
							<div class="flex flex-col gap-1.5">
								<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
									<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
									{failure.key} is already used in this workspace.
								</p>
								{#if failure.suggestions.length > 0}
									<div class="flex flex-wrap items-center gap-1.5">
										<span class="text-sm text-muted-foreground">Free:</span>
										{#each failure.suggestions as suggestion (suggestion)}
											<Button
												variant="chip"
												size="chip"
												onclick={() => pickKey(suggestion)}
											>
												{suggestion}
											</Button>
										{/each}
									</div>
								{/if}
							</div>
						{:else}
							<p class="text-sm leading-normal text-muted-foreground">
								Issues will be numbered
								<span class="font-mono text-ink-600">{$formData.key || "MOB"}-1</span>,
								<span class="font-mono text-ink-600">{$formData.key || "MOB"}-2</span>, and so on.
								Archiving a team never releases its key.
							</p>
						{/if}

						<Form.Field {form} name="visibility">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Who can see it</Form.Label>
									<Select.Root
										type="single"
										name={props.name}
										value={$formData.visibility}
										disabled={busy}
										onValueChange={(value) =>
											($formData.visibility = value as TeamVisibility)}
									>
										<Select.Trigger {...props}>
											{visibilityLabels[$formData.visibility]}
										</Select.Trigger>
										<Select.Content>
											{#each visibilities as visibility (visibility)}
												<Select.Item value={visibility} label={visibilityLabels[visibility]}>
													{visibilityLabels[visibility]}
												</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								{visibilityNotes[$formData.visibility]}
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>
					</form>

					<div class="flex flex-wrap gap-2">
						<Button type="submit" form={formId} disabled={busy}>
							{busy ? "Creating team" : "Create team"}
						</Button>
						{#if listing.kind !== "empty"}
							<Button variant="ghost" href={workspacePath(slug, "/settings/teams")} disabled={busy}>
								Cancel
							</Button>
						{/if}
					</div>
				</section>
			{/if}

			{#if listing.kind === "loading"}
				<ul class="flex flex-col gap-px" aria-busy="true">
					{#each [0, 1, 2] as row (row)}
						<li class="h-12 animate-pulse rounded-md bg-paper-2"></li>
					{/each}
				</ul>
			{:else if listing.kind === "empty"}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					This workspace has no teams yet. Every issue belongs to one, so start with the group you
					work in most.
				</p>
			{:else if listing.kind !== "unavailable"}
				<section class="flex flex-col gap-3">
					<div class="flex items-center gap-3 border-b border-line-default">
						{#each teamListTabs as candidate (candidate)}
							<a
								href={tabHref(candidate)}
								data-active={tab === candidate}
								aria-current={tab === candidate ? "page" : undefined}
								class="relative -mb-px border-b-2 border-transparent pb-1.5 text-sm font-medium text-muted-foreground transition-colors duration-110 ease-out hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring data-[active=true]:border-primary data-[active=true]:text-ink-900"
							>
								{candidate === "active" ? "Active" : "Archived"}
							</a>
						{/each}
					</div>

					{#if shown.length === 0}
						<p class="text-sm leading-normal text-muted-foreground">
							{tab === "archived" ? "Nothing is archived." : "No active teams."}
						</p>
					{:else}
						<ul class="flex flex-col rounded-lg border border-line-default">
							{#each shown as team (team.id)}
								<li class="border-b border-line-subtle last:border-b-0">
									<a
										href={teamHref(team)}
										class="flex flex-wrap items-center gap-2 px-3 py-2.5 transition-colors duration-110 ease-out hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
									>
										{#if team.visibility === "private"}
											<Lock class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
										{:else}
											<Users
												class="size-icon-row shrink-0 text-muted-foreground"
												aria-hidden="true"
											/>
										{/if}
										<span class="min-w-0 flex-[1_1_140px] truncate text-md text-ink-900">
											{team.name}
										</span>
										<TeamKey key={team.key} />
										{#if team.status === "archived"}
											<Tag name="Archived" />
										{/if}
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			{/if}
		</div>
	</div>
</div>
