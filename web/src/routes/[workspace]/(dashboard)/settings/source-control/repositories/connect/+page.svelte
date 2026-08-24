<script lang="ts">
	import { page } from "$app/state";
	import { invalidate } from "$app/navigation";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { keys } from "$lib/api/keys";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import { Checkbox } from "$lib/components/ui/checkbox";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import {
		connectionLabel,
		providerLabel,
		sourceControlPath,
		type SourceControlConnection,
	} from "$lib/source-control/source-control";
	import { connectRepositoriesSchema } from "$lib/source-control/source-control-schema";
	import { connectRepositoriesPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? connectRepositoriesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	const view = $derived(preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);
	const teams = $derived(data.teams);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		validators: zod4Client(connectRepositoriesSchema),
		dataType: "json",
		resetForm: false,
	});

	const { form: fields, enhance, submitting, delayed, message: outcome } = form;

	let typed = $state("");

	const chosen = $derived(view.kind === "choose" ? view.chosen : undefined);
	const offered = $derived(view.kind === "choose" ? view.offered : []);
	const connected = $derived(view.kind === "choose" ? view.connected : []);
	const offerUnreadable = $derived(view.kind === "choose" ? view.offerUnreadable : false);
	const installUrl = $derived(view.kind === "choose" ? view.installUrl : "");
	const installed = $derived(chosen?.authKind === "app" ? chosen : undefined);

	let rereading = $state(false);

	// What the installation reaches is read once, when the screen loads. Somebody granting a
	// repository does it on the forge, in another tab, and comes back to a list that predates
	// the grant — so the offer has to be askable again without a reload nothing here suggests.
	async function reread() {
		rereading = true;

		try {
			await invalidate(keys.page(page.route.id));
		} finally {
			rereading = false;
		}
	}

	const selectable = $derived(offered.filter((one) => !connected.includes(one.fullName)));
	const chosenCount = $derived($fields.fullNames.length);

	function toggle(fullName: string, on: boolean) {
		$fields.fullNames = on
			? [...$fields.fullNames, fullName]
			: $fields.fullNames.filter((one) => one !== fullName);
	}

	function addTyped() {
		const name = typed.trim();

		if (!name || $fields.fullNames.includes(name)) return;

		$fields.fullNames = [...$fields.fullNames, name];
		typed = "";
	}

	function connectionHref(connection: SourceControlConnection): string {
		return `?connection=${connection.id}`;
	}
</script>

<svelte:head><title>Connect a repository · {workspace.name} · Norn</title></svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>
			<a href={sourceControlPath(workspace.slug)} class="hover:text-ink-900">Source control</a>
		</Eyebrow>
		<h1 class="text-lg font-medium tracking-snug text-ink-900">Connect a repository</h1>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Norn watches what you connect here. A branch, commit or pull request naming an issue is
			linked to that issue as it happens.
		</p>
	</div>

	{#if view.kind === "forbidden"}
		<Alert.Root variant="muted">
			<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
			<Alert.Title>You cannot connect repositories here</Alert.Title>
			<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
			<Alert.Title>Source control could not be reached</Alert.Title>
			<Alert.Description>Check your connection and reload.</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "no_connection"}
		<Alert.Root variant="warning">
			<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
			<Alert.Title>No credential is held yet</Alert.Title>
			<Alert.Description>
				Connect the application, or hold a token, before choosing repositories.
				<Button variant="secondary" href={sourceControlPath(workspace.slug)} class="mt-3 self-start">
					Back to source control
				</Button>
			</Alert.Description>
		</Alert.Root>
	{:else}
		{#if $outcome?.kind === "partial"}
			<Alert.Root variant="destructive">
				<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
				<Alert.Title>Some repositories were not connected</Alert.Title>
				<Alert.Description>
					<ul class="mt-1 flex list-disc flex-col gap-1 pl-4">
						{#each $outcome.refused as refusal (refusal)}
							<li>{refusal}</li>
						{/each}
					</ul>
				</Alert.Description>
			</Alert.Root>
		{/if}

		<form method="POST" use:enhance class="flex flex-col gap-6">
			<input type="hidden" name="workspaceId" value={workspace.id} />

			{#if view.connections.length > 1}
				<section class="flex flex-col gap-2 rounded-lg border border-line-subtle p-4">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Credential</h2>
					<ul class="flex flex-col gap-2">
						{#each view.connections as connection (connection.id)}
							<li class="flex items-center justify-between gap-2">
								<span class="truncate text-sm text-ink-900">
									{providerLabel(connection.provider)} · {connectionLabel(connection)}
								</span>
								{#if connection.id === chosen?.id}
									<span class="text-sm text-muted-foreground">Chosen</span>
								{:else}
									<Button variant="secondary" href={connectionHref(connection)}>Use this one</Button>
								{/if}
							</li>
						{/each}
					</ul>
				</section>
			{/if}

			<fieldset class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
				<legend class="px-1 text-md font-medium tracking-snug text-ink-900">Repositories</legend>

				{#if installed}
					<div class="flex flex-wrap items-center gap-2">
						{#if installUrl}
							<Button variant="secondary" size="sm" href={installUrl} rel="external">
								Grant more on {providerLabel(installed.provider)}
							</Button>
						{/if}
						<Button variant="ghost" size="sm" type="button" disabled={rereading} onclick={reread}>
							{rereading ? "Asking…" : "Check again"}
						</Button>
					</div>
				{/if}

				{#if offerUnreadable}
					<Alert.Root variant="warning">
						<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
						<Alert.Title>{providerLabel(chosen?.provider ?? "github")} could not be asked what this credential reaches</Alert.Title>
						<Alert.Description>
							The credential may have stopped working. You can still name a repository yourself
							below, and Norn will check it when you connect.
						</Alert.Description>
					</Alert.Root>
				{:else if selectable.length === 0 && offered.length > 0}
					<p class="text-sm text-muted-foreground text-pretty">
						Everything this credential reaches is already connected. A repository made since is
						reached only once it is granted on {providerLabel(chosen?.provider ?? "github")}.
					</p>
				{:else if offered.length === 0 && installed}
					<p class="text-sm text-muted-foreground text-pretty">
						This installation was granted no repositories. Grant some on
						{providerLabel(installed.provider)}, check again, and they appear here — or name one
						below.
					</p>
				{:else if !installed}
					<p class="text-sm text-muted-foreground text-pretty">
						This credential is a token, and a token cannot be asked what it reaches. Name the
						repository yourself and Norn checks it when you connect.
					</p>
				{/if}

				{#if selectable.length > 0}
					<ul class="flex flex-col gap-1">
						{#each selectable as available (available.externalId)}
							<li>
								<label class="flex items-start gap-2 rounded-md px-1 py-1.5 hover:bg-paper-1">
									<Checkbox
										checked={$fields.fullNames.includes(available.fullName)}
										onCheckedChange={(on) => toggle(available.fullName, on === true)}
										aria-label={available.fullName}
									/>
									<span class="flex min-w-0 flex-col">
										<span class="truncate text-sm text-ink-900">{available.fullName}</span>
										{#if available.private}
											<span class="text-xs text-muted-foreground">Private</span>
										{/if}
									</span>
								</label>
							</li>
						{/each}
					</ul>
				{/if}

				{#if connected.length > 0}
					<p class="text-sm text-muted-foreground">
						Already connected: {connected.join(", ")}.
					</p>
				{/if}

				<div class="flex flex-wrap items-end gap-2">
					<div class="flex min-w-0 flex-1 flex-col gap-1">
						<label for="typed-repository" class="text-sm text-ink-900">
							Or name one yourself
						</label>
						<Input
							id="typed-repository"
							bind:value={typed}
							placeholder="acme/api"
							onkeydown={(event) => {
								if (event.key !== "Enter") return;
								event.preventDefault();
								addTyped();
							}}
						/>
					</div>
					<Button variant="secondary" type="button" onclick={addTyped} disabled={!typed.trim()}>
						Add it
					</Button>
				</div>

				<Form.Field {form} name="fullNames">
					<Form.Control>
						{#snippet children({ props })}
							<input {...props} type="hidden" value={$fields.fullNames.join(",")} />
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				{#if chosenCount > 0}
					<p class="text-sm text-ink-900" aria-live="polite">
						{chosenCount === 1 ? "1 repository chosen" : `${chosenCount} repositories chosen`}: {$fields.fullNames.join(
							", ",
						)}
					</p>
				{/if}
			</fieldset>

			<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
				<h2 class="text-md font-medium tracking-snug text-ink-900">Narrow it to a team</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Left alone, a repository can link an issue on any team, which is what most people want.
					Choose a team only to narrow it — then changes under the path below reach that team and
					nothing else.
				</p>

				<Form.Field {form} name="teamId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Team</Form.Label>
							<select
								{...props}
								bind:value={$fields.teamId}
								class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
							>
								<option value="">Any team</option>
								{#each teams as team (team.id)}
									<option value={team.id}>{team.key} · {team.name}</option>
								{/each}
							</select>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				{#if $fields.teamId}
					<Form.Field {form} name="pathPrefix">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Path</Form.Label>
								<Input {...props} bind:value={$fields.pathPrefix} placeholder="services/api" />
							{/snippet}
						</Form.Control>
						<Form.Description>
							Left empty, everything in the repository goes to that team.
						</Form.Description>
						<Form.FieldErrors />
					</Form.Field>
				{/if}
			</section>

			<Form.Field {form} name="mirrorLabel">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Label to watch for</Form.Label>
						<Input {...props} bind:value={$fields.mirrorLabel} placeholder="norn" />
					{/snippet}
				</Form.Control>
				<Form.Description>
					A platform issue carrying this label is brought across and kept in step. Everything else
					stays where it is.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<input type="hidden" name="connectionId" value={$fields.connectionId} />

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={$submitting || chosenCount === 0}>
					{$delayed ? "Reading the repositories…" : "Connect them"}
				</Button>
				<Button variant="secondary" href={sourceControlPath(workspace.slug)}>Cancel</Button>
			</div>
		</form>
	{/if}
</div>
