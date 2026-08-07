<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import {
		brokenLabel,
		detailOf,
		failureMessage,
		providerLabel,
		sourceControlConnectionPath,
		sourceControlFailure,
		type SourceControlFailure,
		type SourceControlView,
	} from "$lib/source-control/source-control";
	import { connectSourceControlSchema } from "$lib/source-control/source-control-schema";
	import { sourceControlPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? sourceControlPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	let loaded = $state.raw<SourceControlView | undefined>(undefined);
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");

	const view = $derived(loaded ?? preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);

	const form = superForm(defaults(zod4(connectSourceControlSchema)), {
		id: "connect-source-control",
		SPA: true,
		validators: zod4Client(connectSourceControlSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = undefined;
			failureDetail = "";

			const { data: minted, error } = await api.POST(
				"/workspaces/{workspaceId}/source-control/connections",
				{
					params: { path: { workspaceId: workspace.id } },
					body: {
						provider: entered.data.provider,
						repository: entered.data.repository,
						baseUrl: entered.data.baseUrl || undefined,
						teamId: entered.data.teamId || undefined,
						mirrorLabel: entered.data.mirrorLabel || undefined,
						token: entered.data.token,
					},
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);
				const said = detailOf(error, mapped);

				if (mapped.kind === "already_connected") {
					setError(entered, "repository", said);
				} else if (mapped.kind === "credentials_rejected") {
					setError(entered, "token", said);
				} else {
					failure = mapped;
					failureDetail = said;
				}

				return;
			}

			if (!minted) return;

			const existing = view.kind === "list" ? view.connections : [];

			loaded = {
				kind: "connected",
				connections: [minted.connection, ...existing],
				connection: minted.connection,
				webhookUrl: minted.webhookUrl,
				webhookSecret: minted.webhookSecret,
			};
		},
	});

	const { form: fields, enhance, submitting, delayed } = form;

	const shown = $derived(failure ?? preview?.failure);

	const connections = $derived(
		view.kind === "list" || view.kind === "connected" ? view.connections : [],
	);
</script>

<svelte:head><title>Source control · {workspace.name} · Norn</title></svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>Settings</Eyebrow>
		<h1 class="text-lg font-medium tracking-snug text-ink-900">Source control</h1>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Connect a repository and Norn links the branches, commits and pull requests that name an
			issue to that issue. A team can have a merged change move its issue on, and a platform
			issue carrying your label is kept in step with a Norn one, both ways.
		</p>
	</div>

	{#if view.kind === "loading"}
		<p class="text-sm text-muted-foreground">Reading your connections…</p>
	{:else if view.kind === "forbidden"}
		<Alert.Root>
			<Alert.Title>You cannot manage connections</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "forbidden" })}</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "sealing_unavailable"}
		<Alert.Root variant="destructive">
			<Alert.Title>A token cannot be stored</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "sealing_unavailable" })}</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<Alert.Title>Source control could not be reached</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "unavailable" })}</Alert.Description>
		</Alert.Root>
	{:else}
		{#if view.kind === "connected"}
			<section
				class="flex flex-col gap-3 rounded-lg border border-line-subtle bg-card p-4"
				aria-live="polite"
			>
				<h2 class="flex items-center gap-2 text-md font-medium tracking-snug text-ink-900">
					<CircleCheck class="size-icon-row shrink-0 text-success" aria-hidden="true" />
					{view.connection.repository} is connected
				</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Norn installed the webhook for you. If your platform blocks that, add it by hand —
					this is the only time the secret is shown.
				</p>
				<dl class="flex flex-col gap-2 text-sm">
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Payload address</dt>
						<dd class="font-mono text-xs break-all text-ink-900">{view.webhookUrl}</dd>
					</div>
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Secret</dt>
						<dd class="font-mono text-xs break-all text-ink-900">{view.webhookSecret}</dd>
					</div>
				</dl>
			</section>
		{/if}

		{#if connections.length === 0}
			<p class="text-sm text-muted-foreground">No repository is connected yet.</p>
		{:else}
			<ul class="flex flex-col gap-3">
				{#each connections as connection (connection.id)}
					<li
						class="flex flex-col gap-2 rounded-lg border border-line-subtle p-4 sm:flex-row sm:items-start sm:justify-between"
					>
						<div class="flex min-w-0 flex-col gap-1">
							<p class="truncate text-sm font-medium text-ink-900">
								{providerLabel(connection.provider)} · {connection.repository}
							</p>
							{#if connection.status === "broken"}
								<p class="flex items-center gap-1.5 text-sm text-destructive">
									<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
									Not working — {brokenLabel(connection)}.
								</p>
							{:else if connection.verifiedAt}
								<p class="text-sm text-muted-foreground">
									Working. Watching for the label “{connection.mirrorLabel}”.
								</p>
							{/if}
						</div>
						<Button
							variant="secondary"
							href={sourceControlConnectionPath(workspace.slug, connection.id)}
						>
							Manage
						</Button>
					</li>
				{/each}
			</ul>
		{/if}

		{#if shown}
			<Alert.Root variant="destructive">
				<Alert.Title>The repository was not connected</Alert.Title>
				<Alert.Description>{failureDetail || failureMessage(shown)}</Alert.Description>
			</Alert.Root>
		{/if}

		<form method="POST" use:enhance class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Connect a repository</h2>

			<Form.Field {form} name="provider">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Platform</Form.Label>
						<select
							{...props}
							bind:value={$fields.provider}
							class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
						>
							<option value="github">GitHub</option>
							<option value="gitlab">GitLab</option>
						</select>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="repository">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Repository</Form.Label>
						<Input {...props} bind:value={$fields.repository} placeholder="acme/api" />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="baseUrl">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Address</Form.Label>
						<Input {...props} bind:value={$fields.baseUrl} placeholder="Leave empty for the public platform" />
					{/snippet}
				</Form.Control>
				<Form.Description>
					Set this for GitHub Enterprise or a self-hosted GitLab.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="teamId">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Team</Form.Label>
						<select
							{...props}
							bind:value={$fields.teamId}
							class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
						>
							<option value="">The whole workspace</option>
							{#each data.teams as team (team.id)}
								<option value={team.id}>{team.key} · {team.name}</option>
							{/each}
						</select>
					{/snippet}
				</Form.Control>
				<Form.Description>
					A platform issue can only be brought across into a team, so pick one if you want that.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="mirrorLabel">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Label to watch for</Form.Label>
						<Input {...props} bind:value={$fields.mirrorLabel} placeholder="norn" />
					{/snippet}
				</Form.Control>
				<Form.Description>
					A platform issue carrying this label is brought across and kept in step. Everything
					else stays where it is.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="token">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Personal access token</Form.Label>
						<Input {...props} type="password" autocomplete="off" bind:value={$fields.token} />
					{/snippet}
				</Form.Control>
				<Form.Description>
					The connection reaches exactly as far as this token does. It is stored encrypted and
					never shown again.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={$submitting}>
					{$delayed ? "Checking the token…" : "Connect"}
				</Button>
			</div>
		</form>
	{/if}
</div>
