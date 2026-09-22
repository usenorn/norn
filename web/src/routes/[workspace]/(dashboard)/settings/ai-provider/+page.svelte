<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Sparkles from "@lucide/svelte/icons/sparkles";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { deploymentPreview } from "$lib/auth/preview";
	import { onDateAndTime } from "$lib/time";
	import { aiProviderSchema } from "$lib/ai-provider/ai-provider-schema";
	import {
		aiProviderFailure,
		aiProviderKinds,
		failureMessage,
		failureTitle,
		providerLabels,
		type AiProvider,
		type AiProviderFailure,
		type AiProviderKind,
		type AiProviderOutcome,
		type AiProviderView,
	} from "$lib/ai-provider/ai-provider";
	import { aiProviderPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "ai-provider-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? aiProviderPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let liveOutcome = $state<AiProviderOutcome>({ kind: "idle" });
	let testing = $state(false);
	let removing = $state(false);
	let confirmingRemove = $state(false);

	const view = $derived<AiProviderView>(preview?.view ?? data.view);
	const outcome = $derived<AiProviderOutcome>(preview?.outcome ?? liveOutcome);
	const selfHosted = $derived(deploymentPreview(page.url)?.selfHosted ?? data.selfHosted);
	const workspace = $derived(data.workspace);
	const timezone = $derived(page.data.session?.account?.timezone ?? "UTC");
	const provider = $derived<AiProvider | null>(view.kind === "configured" ? view.provider : null);
	const kind = $derived<AiProviderKind>(provider?.provider ?? "openai");
	const label = $derived(providerLabels[kind]);
	const models = $derived(view.kind === "configured" && view.models.kind === "listed" ? view.models.models : []);
	const modelsFailure = $derived<AiProviderFailure | null>(
		view.kind === "configured" && view.models.kind === "failed" ? view.models.failure : null
	);
	const failure = $derived<AiProviderFailure | null>(outcome.kind === "failed" ? outcome.failure : null);
	const recordedFailure = $derived<AiProviderFailure | null>(
		provider?.failure ? { kind: "provider", code: provider.failure } : null
	);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: formId,
		validators: zod4Client(aiProviderSchema),
		resetForm: false,
		onSubmit: () => {
			liveOutcome = { kind: "idle" };
			confirmingRemove = false;
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	$effect(() => {
		const posted = $message;

		if (posted) liveOutcome = posted;
	});

	$effect(() => {
		formData.update(
			(current) => ({
				...current,
				provider: provider?.provider ?? current.provider,
				baseUrl: provider?.baseUrl ?? current.baseUrl,
				allowPrivateAddress: provider?.allowPrivateAddress ?? current.allowPrivateAddress,
				defaultModel: provider?.defaultModel ?? current.defaultModel,
				keyStored: provider !== null,
			}),
			{ taint: false }
		);
	});

	const busy = $derived(testing || removing || $submitting);

	async function test() {
		testing = true;
		liveOutcome = { kind: "idle" };

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/ai-provider/test", {
				params: { path: { workspaceId: workspace.id } },
			});

			await invalidate(keys.page(page.route.id));

			liveOutcome = error ? { kind: "failed", failure: aiProviderFailure(error) } : { kind: "tested" };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			testing = false;
		}
	}

	async function remove() {
		removing = true;
		liveOutcome = { kind: "idle" };

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/ai-provider", {
				params: { path: { workspaceId: workspace.id } },
			});

			if (error) {
				liveOutcome = { kind: "failed", failure: aiProviderFailure(error) };

				return;
			}

			await invalidate(keys.page(page.route.id));
			liveOutcome = { kind: "removed" };
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			removing = false;
			confirmingRemove = false;
		}
	}
</script>

<svelte:head><title>AI provider · {workspace.name} · Norn</title></svelte:head>

<SettingsPage
	title="AI provider"
	description="The model provider this workspace brings its own API key for."
	Icon={Sparkles}
	meta={provider ? `${label} connected` : "not configured"}
	width="compact"
>
	{#if view.kind === "loading"}
		<div class="flex flex-col gap-3" aria-busy="true" aria-label="Loading the AI provider">
			<Skeleton class="h-8 w-56" />
			<Skeleton class="h-48 w-full" />
		</div>
	{:else if view.kind === "forbidden"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>You cannot change this</Alert.Title>
			<Alert.Description>
				Only an administrator of {workspace.name} can configure the AI provider.
			</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Something went wrong</Alert.Title>
			<Alert.Description>We could not read the AI provider settings. Wait a moment and reload.</Alert.Description>
		</Alert.Root>
	{:else}
		<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
			{#if outcome.kind === "saved"}
				<Alert.Root variant="success">
					<CircleCheck aria-hidden="true" />
					<Alert.Title>Saved</Alert.Title>
					<Alert.Description>
						{provider?.defaultModel
							? "Test the connection to confirm the key can reach the model. Saving always clears the last test."
							: "Choose a default model, then test the connection."}
					</Alert.Description>
				</Alert.Root>
			{:else if outcome.kind === "tested"}
				<Alert.Root variant="success">
					<CircleCheck aria-hidden="true" />
					<Alert.Title>The connection works</Alert.Title>
					<Alert.Description>
						{label} answered a request to {provider?.defaultModel} made with this key.
					</Alert.Description>
				</Alert.Root>
			{:else if outcome.kind === "removed"}
				<Alert.Root>
					<CircleCheck aria-hidden="true" />
					<Alert.Title>Key removed</Alert.Title>
					<Alert.Description>The key is gone from {workspace.name}. Nothing here can use it any more.</Alert.Description>
				</Alert.Root>
			{:else if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>{failureTitle(failure, kind)}</Alert.Title>
					<Alert.Description>
						<span class="block">{failureMessage(failure, kind, selfHosted)}</span>
						{#if failure.kind === "provider" && failure.detail}
							<span class="mt-2 block font-mono text-xs break-all">{failure.detail}</span>
						{/if}
					</Alert.Description>
				</Alert.Root>
			{/if}
		</div>

		{#if provider}
			<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4" aria-labelledby="ai-provider-status">
				<h2 id="ai-provider-status" class="text-md font-medium tracking-snug text-ink-900">Status</h2>

				{#if provider.status === "verified"}
					<p class="flex items-center gap-1.5 text-sm text-muted-foreground">
						<CircleCheck class="size-icon-row shrink-0 text-success" aria-hidden="true" />
						Working.
						{#if provider.verifiedAt}
							Last tested {onDateAndTime(provider.verifiedAt, timezone)}.
						{/if}
					</p>
				{:else if provider.status === "failed" && recordedFailure}
					<p class="flex items-start gap-1.5 text-sm text-destructive">
						<TriangleAlert class="mt-0.5 size-icon-row shrink-0" aria-hidden="true" />
						<span>
							{failureTitle(recordedFailure, kind)}.
							{#if provider.failedAt}
								Last tested {onDateAndTime(provider.failedAt, timezone)}.
							{/if}
						</span>
					</p>
				{:else}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Not tested yet. A test sends one small request to the default model, billed to this key.
					</p>
				{/if}

				<dl class="flex flex-col gap-2 text-sm">
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Provider</dt>
						<dd class="text-ink-900">{label}</dd>
					</div>
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Endpoint</dt>
						<dd class="font-mono text-xs break-all text-ink-900">
							{provider.baseUrl || `${label}'s own`}
						</dd>
					</div>
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">API key</dt>
						<dd class="text-ink-900">{provider.keyHint ? `Ending ${provider.keyHint}` : "Stored"}</dd>
					</div>
					<div class="flex flex-col gap-0.5">
						<dt class="text-muted-foreground">Default model</dt>
						<dd class="font-mono text-xs break-all text-ink-900">{provider.defaultModel || "None chosen"}</dd>
					</div>
				</dl>

				<div class="flex flex-wrap items-center gap-2">
					<Button variant="secondary" onclick={test} disabled={busy || !provider.defaultModel}>
						{testing ? "Sending a test request…" : "Test connection"}
					</Button>
					{#if !provider.defaultModel}
						<span class="text-sm text-muted-foreground">Choose a default model to test with.</span>
					{/if}
				</div>
			</section>
		{/if}

		<form
			id={formId}
			method="POST"
			action="?/save"
			use:enhance
			class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4"
			aria-labelledby="ai-provider-key"
		>
			<input type="hidden" name="workspaceId" value={workspace.id} />
			<input type="hidden" name="provider" value={$formData.provider} />
			<input type="hidden" name="defaultModel" value={$formData.defaultModel} />
			<input type="hidden" name="keyStored" value={String($formData.keyStored)} />

			<div class="flex flex-col gap-1">
				<h2 id="ai-provider-key" class="text-md font-medium tracking-snug text-ink-900">
					{provider ? "Change the key or model" : "Connect a provider"}
				</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					The key is proved against the provider before it is stored, and it is stored encrypted. It is
					never shown again, and only {workspace.name} can use it.
				</p>
			</div>

			<Form.Field {form} name="provider">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Provider</Form.Label>
						<Select.Root
							type="single"
							value={$formData.provider}
							disabled={busy}
							onValueChange={(value) => ($formData.provider = value as AiProviderKind)}
						>
							<Select.Trigger {...props} class="w-full">{providerLabels[$formData.provider]}</Select.Trigger>
							<Select.Content>
								{#each aiProviderKinds as option (option)}
									<Select.Item value={option} label={providerLabels[option]}>{providerLabels[option]}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="baseUrl">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Endpoint</Form.Label>
						<Input
							{...props}
							type="url"
							inputmode="url"
							autocapitalize="none"
							spellcheck="false"
							placeholder="Leave blank for {label}'s own"
							disabled={busy}
							bind:value={$formData.baseUrl}
						/>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					Any endpoint that speaks the {label} API, such as a gateway or your own model server. Changing it
					needs the key pasted again.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			{#if selfHosted}
				<Form.Field {form} name="allowPrivateAddress">
					<Form.Control>
						{#snippet children({ props })}
							<div class="flex items-start gap-2">
								<Checkbox {...props} disabled={busy} bind:checked={$formData.allowPrivateAddress} />
								<div class="flex flex-col gap-0.5">
									<Form.Label>Allow an address on a private network</Form.Label>
									<span class="text-sm leading-normal text-muted-foreground text-pretty">
										For a model server inside your network. Loopback and link-local addresses stay refused
										unless NORN_OPENAI_ALLOWED_DESTINATIONS names them.
									</span>
								</div>
							</div>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>
			{/if}

			<Form.Field {form} name="apiKey">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>API key</Form.Label>
						<Input
							{...props}
							type="password"
							autocomplete="off"
							autocapitalize="none"
							spellcheck="false"
							placeholder={provider?.keyHint ? `Stored, ending ${provider.keyHint}. Leave blank to keep it.` : "sk-…"}
							disabled={busy}
							bind:value={$formData.apiKey}
						/>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					{provider
						? "Paste a new key only to replace the stored one. A key the provider refuses never replaces one that works."
						: `Create a key in your ${label} account. It needs permission to list models and make requests.`}
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			{#if provider}
				<Form.Field {form} name="defaultModel">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Default model</Form.Label>
							<Select.Root
								type="single"
								value={$formData.defaultModel}
								disabled={busy || models.length === 0}
								onValueChange={(value) => ($formData.defaultModel = value)}
							>
								<Select.Trigger {...props} class="w-full">
									{$formData.defaultModel || "Choose a model"}
								</Select.Trigger>
								<Select.Content>
									{#each models as model (model)}
										<Select.Item value={model} label={model}>{model}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/snippet}
					</Form.Control>
					<Form.Description class="text-sm text-muted-foreground">
						{#if modelsFailure}
							The models could not be listed. {failureTitle(modelsFailure, kind)}.
							{failureMessage(modelsFailure, kind, selfHosted)}
						{:else}
							The models this key can use, as {provider.baseUrl ? "the endpoint" : label} lists them.
						{/if}
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>
			{:else}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Once the key is saved, choose a default model from the ones it can use.
				</p>
			{/if}

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={busy}>
					{$submitting ? "Checking the key…" : provider ? "Save changes" : "Save key"}
				</Button>
			</div>
		</form>

		{#if provider}
			<section class="flex flex-col gap-3 rounded-lg border border-destructive/40 p-4" aria-labelledby="ai-provider-remove">
				<h2 id="ai-provider-remove" class="text-md font-medium tracking-snug text-ink-900">Remove the key</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					The key is deleted from Norn. It is not revoked at {label}; do that in your {label} account if it
					should stop working everywhere.
				</p>

				{#if confirmingRemove}
					<div class="flex flex-wrap gap-2">
						<Button variant="destructive" onclick={remove} disabled={busy}>
							{removing ? "Removing…" : "Yes, remove it"}
						</Button>
						<Button variant="secondary" onclick={() => (confirmingRemove = false)} disabled={busy}>Keep it</Button>
					</div>
				{:else}
					<Button variant="secondary" onclick={() => (confirmingRemove = true)} disabled={busy} class="self-start">
						Remove key
					</Button>
				{/if}
			</section>
		{/if}
	{/if}
</SettingsPage>
