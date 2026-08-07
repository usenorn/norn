<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import {
		brokenLabel,
		connectionLabel,
		detailOf,
		failureMessage,
		providerLabel,
		sourceControlFailure,
		sourceControlPath,
		sourceControlRepositoryPath,
		type SourceControlConnection,
		type SourceControlDetailView,
		type SourceControlFailure,
	} from "$lib/source-control/source-control";
	import { replaceTokenSchema } from "$lib/source-control/source-control-schema";
	import { sourceControlDetailPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? sourceControlDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	let loaded = $state.raw<SourceControlDetailView | undefined>(undefined);
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");
	let verifying = $state(false);
	let disconnecting = $state(false);
	let confirmingDisconnect = $state(false);

	const view = $derived(loaded ?? preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);
	const timezone = $derived(page.data.session?.account?.timezone ?? "UTC");

	const shown = $derived(failure ?? preview?.failure);

	function replace(connection: SourceControlConnection) {
		loaded = {
			kind: "detail",
			connection,
			repositories: view.kind === "detail" ? view.repositories : [],
		};
	}

	function record(error: unknown) {
		const mapped = sourceControlFailure(error as never);

		failure = mapped;
		failureDetail = detailOf(error as never, mapped);
	}

	const tokenForm = superForm(defaults(zod4(replaceTokenSchema)), {
		id: "replace-source-control-token",
		SPA: true,
		validators: zod4Client(replaceTokenSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || view.kind !== "detail") return;

			failure = undefined;
			failureDetail = "";

			const { data: updated, error } = await api.PUT(
				"/workspaces/{workspaceId}/source-control/connections/{connectionId}/token",
				{
					params: {
						path: { workspaceId: workspace.id, connectionId: view.connection.id },
					},
					body: { token: entered.data.token },
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);

				setError(entered, "token", detailOf(error, mapped));

				return;
			}

			if (updated) replace(updated);
		},
	});

	const { form: tokenFields, enhance: tokenEnhance, submitting: replacing } = tokenForm;

	async function verify() {
		if (view.kind !== "detail") return;

		verifying = true;
		failure = undefined;
		failureDetail = "";

		const { data: verified, error } = await api.POST(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}/verify",
			{
				params: { path: { workspaceId: workspace.id, connectionId: view.connection.id } },
			},
		);

		verifying = false;

		if (error) {
			record(error);

			return;
		}

		if (verified) replace(verified);
	}

	async function disconnect() {
		if (view.kind !== "detail") return;

		disconnecting = true;
		failure = undefined;
		failureDetail = "";

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}",
			{
				params: { path: { workspaceId: workspace.id, connectionId: view.connection.id } },
			},
		);

		disconnecting = false;

		if (error) {
			record(error);

			return;
		}

		await goto(sourceControlPath(workspace.slug));
	}
</script>

<svelte:head>
	<title>
		{view.kind === "detail" ? connectionLabel(view.connection) : "Connection"} · Source control ·
		{workspace.name} · Norn
	</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>
			<a href={sourceControlPath(workspace.slug)} class="hover:text-ink-900">Source control</a>
		</Eyebrow>
		<h1 class="text-lg font-medium tracking-snug text-ink-900">
			{#if view.kind === "detail"}
				{providerLabel(view.connection.provider)} · {connectionLabel(view.connection)}
			{:else}
				Connection
			{/if}
		</h1>
	</div>

	{#if view.kind === "loading"}
		<p class="text-sm text-muted-foreground">Reading this connection…</p>
	{:else if view.kind === "not_found"}
		<Alert.Root>
			<Alert.Title>That connection is gone</Alert.Title>
			<Alert.Description>
				It may have been removed. <a href={sourceControlPath(workspace.slug)} class="underline">
					Back to source control
				</a>.
			</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "forbidden"}
		<Alert.Root>
			<Alert.Title>You cannot manage connections</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "forbidden" })}</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<Alert.Title>Source control could not be reached</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "unavailable" })}</Alert.Description>
		</Alert.Root>
	{:else}
		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Health</h2>

			{#if view.connection.status === "broken"}
				<p class="flex items-start gap-1.5 text-sm text-destructive">
					<TriangleAlert class="mt-0.5 size-icon-row shrink-0" aria-hidden="true" />
					<span>
						Not working — {brokenLabel(view.connection)}.
						{#if view.connection.brokenDetail}
							{view.connection.brokenDetail}
						{/if}
					</span>
				</p>
			{:else}
				<p class="flex items-center gap-1.5 text-sm text-muted-foreground">
					<CircleCheck class="size-icon-row shrink-0 text-success" aria-hidden="true" />
					Working.
					{#if view.connection.verifiedAt}
						Last proved {onDateAndTime(view.connection.verifiedAt, timezone)}.
					{/if}
				</p>
			{/if}

			<dl class="flex flex-col gap-2 text-sm">
				{#if view.connection.identityLogin}
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Acting as</dt>
						<dd class="text-ink-900">{view.connection.identityLogin}</dd>
					</div>
				{/if}
				<div class="flex flex-col gap-0.5">
					<dt class="text-muted-foreground">Token</dt>
					<dd class="text-ink-900">Ending {view.connection.tokenHint}</dd>
				</div>
				{#if view.connection.baseUrl}
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Address</dt>
						<dd class="font-mono text-xs break-all text-ink-900">{view.connection.baseUrl}</dd>
					</div>
				{/if}
			</dl>

			<div class="flex flex-wrap gap-2">
				<Button variant="secondary" onclick={verify} disabled={verifying}>
					{verifying ? "Asking the platform…" : "Check it now"}
				</Button>
			</div>
		</section>

		{#if shown}
			<Alert.Root variant="destructive">
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failureDetail || failureMessage(shown)}</Alert.Description>
			</Alert.Root>
		{/if}

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Repositories</h2>

			{#if view.repositories.length === 0}
				<p class="text-sm text-muted-foreground">
					This credential reaches no repository yet.
					<a href={sourceControlPath(workspace.slug)} class="underline">Connect one</a>.
				</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each view.repositories as repository (repository.id)}
						<li class="flex items-center justify-between gap-2">
							<a
								href={sourceControlRepositoryPath(workspace.slug, repository.id)}
								class="truncate text-sm text-ink-900 underline-offset-2 hover:underline"
							>
								{repository.fullName}
							</a>
							{#if !repository.hookInstalled}
								<span class="shrink-0 text-xs text-muted-foreground">no webhook</span>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<form
			method="POST"
			use:tokenEnhance
			class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4"
		>
			<h2 class="text-md font-medium tracking-snug text-ink-900">Replace the token</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				The new token is proved against the platform before the old one is discarded.
			</p>

			<Form.Field form={tokenForm} name="token">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Personal access token</Form.Label>
						<Input {...props} type="password" autocomplete="off" bind:value={$tokenFields.token} />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={$replacing}>
					{$replacing ? "Checking the token…" : "Replace it"}
				</Button>
			</div>
		</form>

		<section class="flex flex-col gap-3 rounded-lg border border-destructive/40 p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Stop using this credential</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Every repository under it is disconnected and its webhook removed. Links and mirrors
				already on issues stay exactly as readable; the stored token is destroyed.
			</p>

			{#if confirmingDisconnect}
				<div class="flex flex-wrap gap-2">
					<Button variant="destructive" onclick={disconnect} disabled={disconnecting}>
						{disconnecting ? "Disconnecting…" : "Yes, disconnect"}
					</Button>
					<Button
						variant="secondary"
						onclick={() => (confirmingDisconnect = false)}
						disabled={disconnecting}
					>
						Keep it
					</Button>
				</div>
			{:else}
				<Button
					variant="secondary"
					onclick={() => (confirmingDisconnect = true)}
					class="self-start"
				>
					Disconnect
				</Button>
			{/if}
		</section>
	{/if}
</div>
