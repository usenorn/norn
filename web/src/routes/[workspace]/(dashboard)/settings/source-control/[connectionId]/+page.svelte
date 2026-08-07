<script lang="ts">
	import { goto } from "$app/navigation";
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
		failureMessage,
		providerLabel,
		sourceControlFailure,
		sourceControlPath,
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
	let verifying = $state(false);
	let disconnecting = $state(false);
	let confirmingDisconnect = $state(false);

	const view = $derived(loaded ?? preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);

	function replace(connection: SourceControlConnection) {
		loaded = { kind: "detail", connection, links: view.kind === "detail" ? view.links : [] };
	}

	const form = superForm(defaults(zod4(replaceTokenSchema)), {
		id: "replace-source-control-token",
		SPA: true,
		validators: zod4Client(replaceTokenSchema),
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || view.kind !== "detail") return;

			failure = undefined;

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

				if (mapped.kind === "credentials_rejected" || mapped.kind === "repository_unreachable") {
					setError(entered, "token", failureMessage(mapped));
				} else {
					failure = mapped;
				}

				return;
			}

			if (updated) replace(updated);
		},
	});

	const { form: fields, enhance, submitting, delayed } = form;

	async function verify() {
		if (view.kind !== "detail") return;

		verifying = true;
		failure = undefined;

		const { data: checked, error } = await api.POST(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}/verify",
			{ params: { path: { workspaceId: workspace.id, connectionId: view.connection.id } } },
		);

		verifying = false;

		if (error) {
			failure = sourceControlFailure(error);

			return;
		}

		if (checked) replace(checked);
	}

	async function disconnect() {
		if (view.kind !== "detail") return;

		disconnecting = true;
		failure = undefined;

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}",
			{ params: { path: { workspaceId: workspace.id, connectionId: view.connection.id } } },
		);

		disconnecting = false;

		if (error) {
			failure = sourceControlFailure(error);

			return;
		}

		await goto(sourceControlPath(workspace.slug));
	}
</script>

<svelte:head><title>Connection · {workspace.name} · Norn</title></svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>Source control</Eyebrow>
		{#if view.kind === "detail"}
			<h1 class="text-lg font-medium tracking-snug break-words text-ink-900">
				{providerLabel(view.connection.provider)} · {view.connection.repository}
			</h1>
		{:else}
			<h1 class="text-lg font-medium tracking-snug text-ink-900">Connection</h1>
		{/if}
	</div>

	{#if view.kind === "loading"}
		<p class="text-sm text-muted-foreground">Reading the connection…</p>
	{:else if view.kind === "not_found"}
		<Alert.Root>
			<Alert.Title>That connection is gone</Alert.Title>
			<Alert.Description>
				It may have been disconnected. The links it made are still on their issues.
			</Alert.Description>
		</Alert.Root>
		<div><Button variant="secondary" href={sourceControlPath(workspace.slug)}>Back</Button></div>
	{:else if view.kind === "forbidden"}
		<Alert.Root>
			<Alert.Title>You cannot manage this connection</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "forbidden" })}</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<Alert.Title>The connection could not be read</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "unavailable" })}</Alert.Description>
		</Alert.Root>
	{:else}
		{#if view.connection.status === "broken"}
			<Alert.Root variant="destructive">
				<Alert.Title>This connection is not working</Alert.Title>
				<Alert.Description>
					{providerLabel(view.connection.provider)} stopped answering because {brokenLabel(
						view.connection,
					)}. Issues, and every link already made, carry on working. Replace the token below to
					start it again.
				</Alert.Description>
			</Alert.Root>
		{:else if !view.connection.hookInstalled}
			<Alert.Root>
				<Alert.Title>The webhook is not installed yet</Alert.Title>
				<Alert.Description>
					Norn could not add it, so events are only picked up when it next checks. It will keep
					trying.
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if failure}
			<Alert.Root variant="destructive">
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failureMessage(failure)}</Alert.Description>
			</Alert.Root>
		{/if}

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">What is stored</h2>
			<dl class="grid gap-3 sm:grid-cols-2">
				<div class="flex flex-col gap-0.5">
					<dt class="text-sm text-muted-foreground">Token</dt>
					<dd class="text-sm text-ink-900">
						{#if view.connection.tokenSet}
							A token is stored{#if view.connection.tokenHint}, ending {view.connection
									.tokenHint}{/if}.
						{:else}
							None.
						{/if}
					</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-sm text-muted-foreground">Acting as</dt>
					<dd class="text-sm text-ink-900">{view.connection.identityLogin || "Not known yet"}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-sm text-muted-foreground">Label watched for</dt>
					<dd class="text-sm text-ink-900">{view.connection.mirrorLabel}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-sm text-muted-foreground">Address</dt>
					<dd class="text-sm break-all text-ink-900">
						{view.connection.baseUrl || `The public ${providerLabel(view.connection.provider)}`}
					</dd>
				</div>
			</dl>

			<div class="flex flex-wrap gap-2">
				<Button variant="secondary" disabled={verifying} onclick={verify}>
					{verifying ? "Asking the platform…" : "Check it still works"}
				</Button>
				{#if view.connection.status === "connected" && view.connection.verifiedAt}
					<p class="flex items-center gap-1.5 text-sm text-muted-foreground">
						<CircleCheck class="size-icon-row shrink-0 text-success" aria-hidden="true" />
						Answering
					</p>
				{/if}
			</div>
		</section>

		<form method="POST" use:enhance class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Replace the token</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				The new one is tried against the repository before it is kept, so a token that does not
				work leaves the old state alone rather than reporting success.
			</p>

			<Form.Field {form} name="token">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Personal access token</Form.Label>
						<Input {...props} type="password" autocomplete="off" bind:value={$fields.token} />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<div>
				<Button type="submit" disabled={$submitting}>
					{$delayed ? "Checking the token…" : "Replace"}
				</Button>
			</div>
		</form>

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Disconnect</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Norn removes the webhook and destroys the stored token. Every branch, commit and change
				already linked stays on its issue and keeps its address — you simply stop getting new
				ones.
			</p>

			{#if confirmingDisconnect}
				<div class="flex flex-wrap items-center gap-2">
					<p class="flex items-center gap-1.5 text-sm text-ink-900">
						<TriangleAlert class="size-icon-row shrink-0 text-destructive" aria-hidden="true" />
						Disconnect {view.connection.repository}?
					</p>
					<Button variant="destructive" disabled={disconnecting} onclick={disconnect}>
						{disconnecting ? "Disconnecting…" : "Yes, disconnect"}
					</Button>
					<Button
						variant="ghost"
						disabled={disconnecting}
						onclick={() => (confirmingDisconnect = false)}
					>
						Keep it
					</Button>
				</div>
			{:else}
				<div>
					<Button variant="secondary" onclick={() => (confirmingDisconnect = true)}>
						Disconnect
					</Button>
				</div>
			{/if}
		</section>

		<div><Button variant="ghost" href={sourceControlPath(workspace.slug)}>Back</Button></div>
	{/if}
</div>
