<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import ShieldCheck from "@lucide/svelte/icons/shield-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { api } from "$lib/api";
	import { onDate } from "$lib/time";
	import { ssoConnectionSchema } from "$lib/workspace/sso-schema";
	import EnforcementPanel from "$lib/workspace/enforcement-panel.svelte";
	import SamlPanel from "$lib/workspace/saml-panel.svelte";
	import {
		failureMessage,
		failureTitle,
		parseScopes,
		saveFailure,
		scopeText,
		stageAdvice,
		type OidcConnection,
		type SamlConnection,
		type SsoFailure,
		type SsoOutcome,
		type Enforcement,
		type RecoveryCodes,
		type SsoProtocol,
		type SsoProviderConfiguration,
	} from "$lib/workspace/sso";
	import { authenticationPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "sso-connection-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? authenticationPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let saved = $state<SsoProviderConfiguration | null>(null);
	let chosen = $state<SsoProtocol | null>(null);
	let samlBusy = $state(false);
	let policyBusy = $state(false);
	let liveOutcome = $state<SsoOutcome>(
		page.url.searchParams.get("tested") === "1" ? { kind: "verified" } : { kind: "idle" }
	);
	let discovering = $state(false);
	let testing = $state(false);
	let removing = $state(false);

	const configuration = $derived<SsoProviderConfiguration>(
		saved ?? preview?.configuration ?? data.configuration
	);
	const outcome = $derived<SsoOutcome>(preview?.outcome ?? liveOutcome);
	const workspace = $derived(data.workspace);
	const connection = $derived<OidcConnection | null>(
		configuration.kind === "oidc" ? configuration.connection : null
	);
	const samlConnection = $derived<SamlConnection | null>(
		configuration.kind === "saml" ? configuration.connection : null
	);
	const configured = $derived<SsoProtocol | null>(
		configuration.kind === "oidc" ? "oidc" : configuration.kind === "saml" ? "saml" : null
	);
	const protocol = $derived<SsoProtocol>(chosen ?? configured ?? "oidc");
	const replacing = $derived(configured !== null && configured !== protocol);
	const redirectUri = $derived(
		connection?.redirectUri ?? `${page.url.origin}/v1/sso/oidc/callback`
	);
	const entryPoint = $derived(`${page.url.origin}/sso?workspace=${workspace.slug}`);
	const anyConnection = $derived(connection !== null || samlConnection !== null);
	const failure = $derived<SsoFailure | null>(
		outcome.kind === "failed" ? outcome.failure : null
	);

	const form = superForm(defaults(zod4(ssoConnectionSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(ssoConnectionSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			liveOutcome = { kind: "idle" };

			try {
				const { data: result, error } = await api.PUT("/workspaces/{workspaceId}/sso/oidc", {
					params: { path: { workspaceId: workspace.id } },
					body: {
						issuer: entered.data.issuer,
						endpoints: entered.data.manual
							? {
									issuer: entered.data.issuer,
									authorizationEndpoint: entered.data.authorizationEndpoint,
									tokenEndpoint: entered.data.tokenEndpoint,
									jwksUri: entered.data.jwksUri,
									userinfoEndpoint: entered.data.userinfoEndpoint || undefined,
								}
							: undefined,
						clientId: entered.data.clientId,
						clientSecret: entered.data.clientSecret || undefined,
						scopes: parseScopes(entered.data.scopes),
						groupsClaim: entered.data.groupsClaim || undefined,
						provisioning: entered.data.provisioning,
					},
				});

				if (error) {
					const mapped = saveFailure(error);

					if (mapped.kind === "stage" && mapped.stage === "discovery") {
						setError(entered, "issuer", failureMessage(mapped));
					}

					liveOutcome = { kind: "failed", failure: mapped };

					return;
				}

				if (!result) {
					liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };

					return;
				}

				saved = { kind: "oidc", connection: result };
				liveOutcome = { kind: "saved" };
				formData.update((current) => ({ ...current, clientSecret: "" }), { taint: false });
				await invalidate(keys.page(page.route.id));
			} catch {
				liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
			}
		},
	});
	const { form: formData, errors, enhance, submitting } = form;

	const busy = $derived(
		discovering || testing || removing || samlBusy || policyBusy || (preview?.discovering ?? false) || $submitting
	);

	$effect(() => {
		if (!connection) return;

		formData.update(
			(current) => ({
				...current,
				issuer: connection.endpoints.issuer,
				manual: !connection.discovered,
				authorizationEndpoint: connection.endpoints.authorizationEndpoint,
				tokenEndpoint: connection.endpoints.tokenEndpoint,
				jwksUri: connection.endpoints.jwksUri,
				userinfoEndpoint: connection.endpoints.userinfoEndpoint ?? "",
				clientId: connection.clientId,
				scopes: scopeText(connection.scopes),
				groupsClaim: connection.groupsClaim ?? "",
				provisioning: connection.provisioning,
			}),
			{ taint: false }
		);
	});

	async function discover() {
		if (!$formData.issuer) {
			$errors.issuer = ["Enter the issuer URL from your provider."];

			return;
		}

		discovering = true;
		liveOutcome = { kind: "idle" };

		try {
			const { data: endpoints, error } = await api.POST(
				"/workspaces/{workspaceId}/sso/oidc/discover",
				{
					params: { path: { workspaceId: workspace.id } },
					body: { issuer: $formData.issuer },
				}
			);

			if (error) {
				liveOutcome = { kind: "failed", failure: saveFailure(error) };

				return;
			}

			if (!endpoints) return;

			formData.update((current) => ({
				...current,
				manual: false,
				issuer: endpoints.issuer || current.issuer,
				authorizationEndpoint: endpoints.authorizationEndpoint,
				tokenEndpoint: endpoints.tokenEndpoint,
				jwksUri: endpoints.jwksUri,
				userinfoEndpoint: endpoints.userinfoEndpoint ?? "",
			}));
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			discovering = false;
		}
	}

	async function test() {
		testing = true;
		liveOutcome = { kind: "idle" };

		try {
			const { data: authorization, error } = await api.POST(
				"/workspaces/{workspaceId}/sso/oidc/test",
				{ params: { path: { workspaceId: workspace.id } } }
			);

			if (error) {
				liveOutcome = { kind: "failed", failure: saveFailure(error) };

				return;
			}

			if (authorization) window.location.assign(authorization.authorizationUrl);
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
			const { error } = await api.DELETE("/workspaces/{workspaceId}/sso", {
				params: { path: { workspaceId: workspace.id } },
			});

			if (error) {
				liveOutcome = { kind: "failed", failure: saveFailure(error) };

				return;
			}

			saved = { kind: "unconfigured" };
			liveOutcome = { kind: "removed" };
			await invalidate(keys.page(page.route.id));
		} catch {
			liveOutcome = { kind: "failed", failure: { kind: "unavailable" } };
		} finally {
			removing = false;
		}
	}
</script>

<svelte:head><title>Authentication · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<KeyRound class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">
				Authentication
			</h1>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if configuration.kind === "loading"}
				<p class="text-md text-muted-foreground" aria-live="polite">Loading…</p>
			{:else if configuration.kind === "forbidden"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>You cannot change this</Alert.Title>
					<Alert.Description>
						Only an administrator of {workspace.name} can set up single sign-on.
					</Alert.Description>
				</Alert.Root>
			{:else if configuration.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>Something went wrong</Alert.Title>
					<Alert.Description>
						We could not read the single sign-on settings. Wait a moment and reload.
					</Alert.Description>
				</Alert.Root>
			{:else}
				{#if outcome.kind === "verified"}
					<Alert.Root variant="success">
						<ShieldCheck aria-hidden="true" />
						<Alert.Title>The connection works</Alert.Title>
						<Alert.Description>
							Norn completed a full round trip to your provider and read a valid identity back.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if outcome.kind === "saved"}
					<Alert.Root variant="success">
						<CircleCheck aria-hidden="true" />
						<Alert.Title>Provider saved</Alert.Title>
						<Alert.Description>
							Test the connection to confirm it works end to end. Saving always clears the last
							test, because a changed setting can break a connection that worked a moment ago.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if outcome.kind === "removed"}
					<Alert.Root>
						<CircleCheck aria-hidden="true" />
						<Alert.Title>Provider removed</Alert.Title>
						<Alert.Description>
							Nobody can sign into {workspace.name} through single sign-on any more. Existing
							sessions are untouched.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if failure}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>{failureTitle(failure)}</Alert.Title>
						<Alert.Description>
							<span class="block">{failureMessage(failure)}</span>
							{#if failure.kind === "stage"}
								<span class="mt-1 block">{stageAdvice[failure.stage]}</span>
								{#if failure.providerMessage}
									<span class="mt-2 block font-mono text-xs break-all">
										{failure.providerMessage}
									</span>
								{/if}
							{/if}
						</Alert.Description>
					</Alert.Root>
				{/if}

				<section class="flex flex-col gap-2">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Protocol</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							A workspace uses one provider. Choosing the other protocol replaces the one
							configured now.
						</p>
					</div>

					<div class="flex flex-wrap gap-2">
						{#each [["oidc", "OpenID Connect"], ["saml", "SAML 2.0"]] as [value, label] (value)}
							<Button
								variant={protocol === value ? "default" : "secondary"}
								size="sm"
								disabled={busy}
								onclick={() => (chosen = value as SsoProtocol)}
							>
								{label}
							</Button>
						{/each}
					</div>

					{#if replacing}
						<Alert.Root variant="warning">
							<TriangleAlert aria-hidden="true" />
							<Alert.Title>This replaces your current provider</Alert.Title>
							<Alert.Description>
								Saving {protocol === "saml" ? "a SAML" : "an OpenID Connect"} provider removes the
								{configured === "saml" ? "SAML" : "OpenID Connect"} one. People already signed in
								stay signed in.
							</Alert.Description>
						</Alert.Root>
					{/if}
				</section>

				<section class="flex flex-col gap-2">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							{protocol === "saml"
								? "Single sign-on with SAML 2.0"
								: "Single sign-on with OpenID Connect"}
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Members of {workspace.name} sign in through your identity provider instead of a Norn
							password. This is available on every Norn instance, including self-hosted, at no
							cost.
						</p>
					</div>

					<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
						<Eyebrow class="text-ink-600">Redirect URI</Eyebrow>
						<span class="font-mono text-sm break-all text-ink-900">{redirectUri}</span>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Register this exactly, in your provider's app configuration, before testing.
						</p>
					</div>

					{#if connection}
						<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
							<Eyebrow class="text-ink-600">Sign-in address</Eyebrow>
							<span class="font-mono text-sm break-all text-ink-900">{entryPoint}</span>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Send members here to sign in through your provider.
							</p>
						</div>
					{/if}
				</section>

				{#if protocol === "saml"}
					<SamlPanel
						{workspace}
						connection={samlConnection}
						busy={busy}
						onfailure={(f) => (liveOutcome = f ? { kind: "failed", failure: f } : { kind: "idle" })}
						onsaved={() => {
							chosen = null;
							liveOutcome = { kind: "saved" };
						}}
						onbusy={(working) => (samlBusy = working)}
					/>
				{:else}
				<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
						<Form.Field {form} name="issuer">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Issuer URL</Form.Label>
									<div class="flex gap-2">
										<Input
											{...props}
											class="flex-1"
											placeholder="https://login.example.com"
											autocapitalize="none"
											spellcheck="false"
											disabled={busy}
											bind:value={$formData.issuer}
										/>
										<Button
											type="button"
											variant="secondary"
											disabled={busy}
											onclick={discover}
										>
											{discovering || preview?.discovering ? "Reading" : "Discover"}
										</Button>
									</div>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								Discover reads /.well-known/openid-configuration and fills in the endpoints below.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="manual">
							<Form.Control>
								{#snippet children({ props })}
									<div class="flex items-start gap-2">
										<Checkbox
											{...props}
											disabled={busy}
											bind:checked={$formData.manual}
										/>
										<div class="flex flex-col gap-0.5">
											<Form.Label>Enter the endpoints by hand</Form.Label>
											<span class="text-sm leading-normal text-muted-foreground text-pretty">
												For a provider that does not publish a discovery document.
											</span>
										</div>
									</div>
								{/snippet}
							</Form.Control>
						</Form.Field>

						{#if $formData.manual}
							<Form.Field {form} name="authorizationEndpoint">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Authorization endpoint</Form.Label>
										<Input
											{...props}
											autocapitalize="none"
											spellcheck="false"
											disabled={busy}
											bind:value={$formData.authorizationEndpoint}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="tokenEndpoint">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Token endpoint</Form.Label>
										<Input
											{...props}
											autocapitalize="none"
											spellcheck="false"
											disabled={busy}
											bind:value={$formData.tokenEndpoint}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="jwksUri">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>JWKS URI</Form.Label>
										<Input
											{...props}
											autocapitalize="none"
											spellcheck="false"
											disabled={busy}
											bind:value={$formData.jwksUri}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="userinfoEndpoint">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Userinfo endpoint</Form.Label>
										<Input
											{...props}
											autocapitalize="none"
											spellcheck="false"
											disabled={busy}
											bind:value={$formData.userinfoEndpoint}
										/>
									{/snippet}
								</Form.Control>
								<Form.Description class="text-sm text-muted-foreground">
									Optional. Norn reads identities from the ID token.
								</Form.Description>
								<Form.FieldErrors />
							</Form.Field>
						{:else if connection}
							<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
								<Eyebrow class="text-ink-600">Discovered endpoints</Eyebrow>
								<dl class="flex flex-col gap-1">
									<div class="flex flex-col">
										<dt class="text-sm text-muted-foreground">Authorization</dt>
										<dd class="font-mono text-xs break-all text-ink-900">
											{connection.endpoints.authorizationEndpoint}
										</dd>
									</div>
									<div class="flex flex-col">
										<dt class="text-sm text-muted-foreground">Token</dt>
										<dd class="font-mono text-xs break-all text-ink-900">
											{connection.endpoints.tokenEndpoint}
										</dd>
									</div>
									<div class="flex flex-col">
										<dt class="text-sm text-muted-foreground">JWKS</dt>
										<dd class="font-mono text-xs break-all text-ink-900">
											{connection.endpoints.jwksUri}
										</dd>
									</div>
								</dl>
							</div>
						{/if}

						<Form.Field {form} name="clientId">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Client ID</Form.Label>
									<Input
										{...props}
										autocapitalize="none"
										spellcheck="false"
										disabled={busy}
										bind:value={$formData.clientId}
									/>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="clientSecret">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Client secret</Form.Label>
									<Input
										{...props}
										type="password"
										autocomplete="off"
										placeholder={connection?.secretSet ? "Stored — leave blank to keep it" : ""}
										disabled={busy}
										bind:value={$formData.clientSecret}
									/>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								{connection?.secretSet
									? "The stored secret is never shown again. Leave this blank unless you are replacing it."
									: "Stored encrypted. It is never returned by the API."}
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="scopes">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Scopes</Form.Label>
									<Input
										{...props}
										autocapitalize="none"
										spellcheck="false"
										disabled={busy}
										bind:value={$formData.scopes}
									/>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								Space separated. openid is always requested whether or not you list it.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="groupsClaim">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Groups claim</Form.Label>
									<Input
										{...props}
										autocapitalize="none"
										spellcheck="false"
										placeholder="groups"
										disabled={busy}
										bind:value={$formData.groupsClaim}
									/>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								Optional. Groups are read and stored; mapping them to teams is not built yet.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="provisioning">
							<Form.Control>
								{#snippet children({ props })}
									<div class="flex items-start gap-2">
										<Checkbox
											{...props}
											disabled={busy}
											bind:checked={$formData.provisioning}
										/>
										<div class="flex flex-col gap-0.5">
											<Form.Label>Create accounts on first sign-in</Form.Label>
											<span class="text-sm leading-normal text-muted-foreground text-pretty">
												Anyone your provider vouches for gets a Norn account and joins {workspace.name}
												as a member. With this off, only people already invited can sign in.
											</span>
										</div>
									</div>
								{/snippet}
							</Form.Control>
						</Form.Field>
					</form>

					<div class="flex flex-wrap gap-2">
						<Button type="submit" form={formId} disabled={busy}>
							{$submitting ? "Saving" : connection ? "Save changes" : "Save provider"}
						</Button>

						{#if connection}
							<Button type="button" variant="secondary" disabled={busy} onclick={test}>
								{testing ? "Opening your provider" : "Test connection"}
							</Button>
						{/if}
					</div>

				{/if}

				{#if anyConnection}
					<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
						<Eyebrow class="text-ink-600">Status</Eyebrow>
						{#if connection?.verifiedAt ?? samlConnection?.verifiedAt}
							<p class="text-md leading-normal text-ink-900">
								Tested successfully on {onDate((connection?.verifiedAt ?? samlConnection?.verifiedAt)!, workspace.timezone)}.
							</p>
						{:else}
							<p class="text-md leading-normal text-ink-900">Not tested yet.</p>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Test the connection before you tell anyone to use it. A test is a real round trip
								through your provider and back.
							</p>
						{/if}
					</div>
				{/if}

				{#if anyConnection}
					<EnforcementPanel
						{workspace}
						enforcement={preview?.enforcement ?? data.enforcement}
						codes={preview?.codes ?? { kind: "none" }}
						{busy}
						onbusy={(working) => (policyBusy = working)}
					/>
				{/if}

				{#if anyConnection}
					<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">Remove this provider</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Single sign-on stops working for {workspace.name} immediately. Anyone who has no
								password will need one before they can sign in again.
							</p>
						</div>

						<div>
							<Button variant="destructive" disabled={busy} onclick={remove}>
								{removing ? "Removing" : "Remove provider"}
							</Button>
						</div>
					</section>
				{/if}
			{/if}
		</div>
	</div>
</div>
