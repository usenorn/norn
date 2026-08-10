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
	import { Checkbox } from "$lib/components/ui/checkbox";
	import { Input } from "$lib/components/ui/input";
	import { Textarea } from "$lib/components/ui/textarea";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import GithubAppPanel from "$lib/source-control/github-app-panel.svelte";
	import {
		brokenLabel,
		connectionLabel,
		detailOf,
		failureMessage,
		providerLabel,
		requiresAddress,
		sourceControlConnectionPath,
		sourceControlFailure,
		sourceControlIdentitiesPath,
		sourceControlRepositoryPath,
		type AvailableSourceControlRepository,
		type MintedRepository,
		type SourceControlFailure,
		type SourceControlView,
	} from "$lib/source-control/source-control";
	import {
		addRepositorySchema,
		connectSourceControlSchema,
	} from "$lib/source-control/source-control-schema";
	import { sourceControlPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? sourceControlPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	let loaded = $state.raw<SourceControlView | undefined>(undefined);
	let minted = $state.raw<MintedRepository | undefined>(undefined);
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");

	const view = $derived(loaded ?? preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);

	const connections = $derived(view.kind === "list" ? view.connections : []);
	const repositories = $derived(view.kind === "list" ? view.repositories : []);

	const connectForm = superForm(defaults(zod4(connectSourceControlSchema)), {
		id: "connect-source-control",
		SPA: true,
		validators: zod4Client(connectSourceControlSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = undefined;
			failureDetail = "";

			const { data: created, error } = await api.POST(
				"/workspaces/{workspaceId}/source-control/connections",
				{
					params: { path: { workspaceId: workspace.id } },
					body: {
						provider: entered.data.provider,
						baseUrl: entered.data.baseUrl || undefined,
						label: entered.data.label || undefined,
						token: entered.data.token,
					},
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);
				const said = detailOf(error, mapped);

				if (mapped.kind === "credentials_rejected") {
					setError(entered, "token", said);
				} else if (mapped.kind === "already_connected") {
					setError(entered, "baseUrl", said);
				} else {
					failure = mapped;
					failureDetail = said;
				}

				return;
			}

			if (!created) return;

			loaded = {
				kind: "list",
				connections: [created, ...connections],
				repositories,
			};
		},
	});

	const {
		form: connectFields,
		enhance: connectEnhance,
		submitting: connecting,
		delayed: connectDelayed,
	} = connectForm;

	const repositoryForm = superForm(defaults(zod4(addRepositorySchema)), {
		id: "add-source-control-repository",
		SPA: true,
		validators: zod4Client(addRepositorySchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = undefined;
			failureDetail = "";

			const { data: added, error } = await api.POST(
				"/workspaces/{workspaceId}/source-control/repositories",
				{
					params: { path: { workspaceId: workspace.id } },
					body: {
						connectionId: entered.data.connectionId,
						fullName: entered.data.fullName,
						mirrorLabel: entered.data.mirrorLabel || undefined,
					},
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);
				const said = detailOf(error, mapped);

				if (mapped.kind === "already_connected" || mapped.kind === "repository_unreachable") {
					setError(entered, "fullName", said);
				} else {
					failure = mapped;
					failureDetail = said;
				}

				return;
			}

			if (!added) return;

			minted = added;
			loaded = {
				kind: "list",
				connections,
				repositories: [added.repository, ...repositories],
			};
		},
	});

	const {
		form: repositoryFields,
		enhance: repositoryEnhance,
		submitting: adding,
		delayed: addingDelayed,
	} = repositoryForm;

	const shown = $derived(failure ?? preview?.failure);
	const shownMinted = $derived(minted ?? preview?.minted);
	const shownApplication = $derived(preview?.application ?? data.application);
	const shownNotice = $derived(preview?.notice ?? data.notice);

	function deliversCentrally(connectionId: string): boolean {
		return connections.some(
			(connection) => connection.id === connectionId && connection.authKind === "app",
		);
	}

	const chosenConnection = $derived(
		connections.find((connection) => connection.id === $repositoryFields.connectionId),
	);

	let offered = $state.raw<AvailableSourceControlRepository[]>([]);
	let offering = $state(false);
	let offerUnreadable = $state(false);

	const offeringConnectionId = $derived(
		chosenConnection?.authKind === "app" ? chosenConnection.id : "",
	);
	const offeringWorkspaceId = $derived(workspace.id);

	$effect(() => {
		if (!offeringConnectionId) {
			offered = [];
			offerUnreadable = false;
			offering = false;

			return;
		}

		let current = true;
		offering = true;
		offerUnreadable = false;

		api
			.GET(
				"/workspaces/{workspaceId}/source-control/connections/{connectionId}/available-repositories",
				{
					params: {
						path: { workspaceId: offeringWorkspaceId, connectionId: offeringConnectionId },
					},
				},
			)
			.then(({ data: reachable, error: unreadable }) => {
				if (!current) return;

				offered = reachable ?? [];
				offerUnreadable = Boolean(unreadable);
				offering = false;
			});

		return () => {
			current = false;
			offering = false;
		};
	});
</script>

<svelte:head><title>Source control · {workspace.name} · Norn</title></svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>Settings</Eyebrow>
		<h1 class="text-lg font-medium tracking-snug text-ink-900">Source control</h1>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Hold one credential per forge, then attach the repositories it reaches. Norn links the
			branches, commits and pull requests that name an issue to that issue, and a route decides
			which team the changes under a path belong to.
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
		{#if shownMinted}
			<section
				class="flex flex-col gap-3 rounded-lg border border-line-subtle bg-card p-4"
				aria-live="polite"
			>
				<h2 class="flex items-center gap-2 text-md font-medium tracking-snug text-ink-900">
					<CircleCheck class="size-icon-row shrink-0 text-success" aria-hidden="true" />
					{shownMinted.repository.fullName} is connected
				</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{#if deliversCentrally(shownMinted.repository.connectionId)}
						Deliveries arrive through the application's own webhook, so this repository needs
						none of its own.
					{:else if shownMinted.repository.hookInstalled}
						Norn installed the webhook for you. If you ever need to add it by hand, this is the
						only time the secret is shown.
					{:else}
						Norn could not install the webhook — the token may not administer this repository.
						Add it by hand; this is the only time the secret is shown.
					{/if}
				</p>
				{#if !deliversCentrally(shownMinted.repository.connectionId)}
					<dl class="flex flex-col gap-2 text-sm">
						<div class="flex flex-col gap-0.5">
							<dt class="text-muted-foreground">Payload address</dt>
							<dd class="font-mono text-xs break-all text-ink-900">{shownMinted.webhookUrl}</dd>
						</div>
						<div class="flex flex-col gap-0.5">
							<dt class="text-muted-foreground">Secret</dt>
							<dd class="font-mono text-xs break-all text-ink-900">{shownMinted.webhookSecret}</dd>
						</div>
					</dl>
				{/if}
				<Button
					variant="secondary"
					href={sourceControlRepositoryPath(workspace.slug, shownMinted.repository.id)}
					class="self-start"
				>
					Route it to a team
				</Button>
			</section>
		{/if}

		{#if shown}
			<Alert.Root variant="destructive">
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failureDetail || failureMessage(shown)}</Alert.Description>
			</Alert.Root>
		{/if}

		<GithubAppPanel
			workspaceId={workspace.id}
			workspaceSlug={workspace.slug}
			application={shownApplication}
			notice={shownNotice}
		/>

		<section class="flex flex-col gap-2 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Platform identities</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Say who each member is on each platform. Until somebody is mapped, an assignee does not
				cross in either direction — Norn will not guess from a handle that resembles a name.
			</p>
			<Button
				variant="secondary"
				href={sourceControlIdentitiesPath(workspace.slug)}
				class="self-start"
			>
				Manage identities
			</Button>
		</section>

		<section class="flex flex-col gap-3">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Connections</h2>

			{#if connections.length === 0}
				<p class="text-sm text-muted-foreground">
					No credential is held yet. Add one below, then attach the repositories it reaches.
				</p>
			{:else}
				<ul class="flex flex-col gap-3">
					{#each connections as connection (connection.id)}
						<li
							class="flex flex-col gap-2 rounded-lg border border-line-subtle p-4 sm:flex-row sm:items-start sm:justify-between"
						>
							<div class="flex min-w-0 flex-col gap-1">
								<p class="truncate text-sm font-medium text-ink-900">
									{providerLabel(connection.provider)} · {connectionLabel(connection)}
								</p>
								{#if connection.status === "broken"}
									<p class="flex items-center gap-1.5 text-sm text-destructive">
										<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
										Not working — {brokenLabel(connection)}.
									</p>
								{:else if connection.verifiedAt && connection.authKind === "app"}
									<p class="text-sm text-muted-foreground">
										Working. Acting as {connection.identityLogin ?? "an installation"}.
									</p>
								{:else if connection.verifiedAt}
									<p class="text-sm text-muted-foreground">
										Working. Token ending {connection.tokenHint}.
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
		</section>

		<section class="flex flex-col gap-3">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Repositories</h2>

			{#if repositories.length === 0}
				<p class="text-sm text-muted-foreground">No repository is connected yet.</p>
			{:else}
				<ul class="flex flex-col gap-3">
					{#each repositories as repository (repository.id)}
						<li
							class="flex flex-col gap-2 rounded-lg border border-line-subtle p-4 sm:flex-row sm:items-start sm:justify-between"
						>
							<div class="flex min-w-0 flex-col gap-1">
								<p class="truncate text-sm font-medium text-ink-900">
									{providerLabel(repository.provider)} · {repository.fullName}
								</p>
								<p class="text-sm text-muted-foreground">
									{#if !repository.hookInstalled && !deliversCentrally(repository.connectionId)}
										The webhook is not installed — the sweep is carrying it.
									{:else if repository.routeCount === 0}
										No team is routed yet, so nothing here reaches an issue.
									{:else}
										Watching for the label “{repository.mirrorLabel}”.
									{/if}
								</p>
							</div>
							<Button
								variant="secondary"
								href={sourceControlRepositoryPath(workspace.slug, repository.id)}
							>
								Manage
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<form
			method="POST"
			use:connectEnhance
			class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4"
		>
			<h2 class="text-md font-medium tracking-snug text-ink-900">Hold a credential</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				GitLab and Gitea have no application to install, so they are reached with a token. GitHub
				accepts one too, until an application is set up.
			</p>

			<Form.Field form={connectForm} name="provider">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Platform</Form.Label>
						<select
							{...props}
							bind:value={$connectFields.provider}
							class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
						>
							<option value="github">GitHub</option>
							<option value="gitlab">GitLab</option>
						</select>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field form={connectForm} name="baseUrl">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Address</Form.Label>
						<Input
							{...props}
							bind:value={$connectFields.baseUrl}
							placeholder="Leave empty for the public platform"
						/>
					{/snippet}
				</Form.Control>
				<Form.Description>
					Set this for GitHub Enterprise or a self-hosted GitLab.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field form={connectForm} name="label">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Name</Form.Label>
						<Input
							{...props}
							bind:value={$connectFields.label}
							placeholder="Whose account this token belongs to"
						/>
					{/snippet}
				</Form.Control>
				<Form.Description>
					Left empty, the account the token belongs to is used.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field form={connectForm} name="token">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Personal access token</Form.Label>
						<Input
							{...props}
							type="password"
							autocomplete="off"
							bind:value={$connectFields.token}
						/>
					{/snippet}
				</Form.Control>
				<Form.Description>
					The connection reaches exactly as far as this token does. It is stored encrypted and
					never shown again.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={$connecting}>
					{$connectDelayed ? "Checking the token…" : "Hold it"}
				</Button>
			</div>
		</form>

		{#if connections.length > 0}
			<form
				method="POST"
				use:repositoryEnhance
				class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4"
			>
				<h2 class="text-md font-medium tracking-snug text-ink-900">Connect a repository</h2>

				<Form.Field form={repositoryForm} name="connectionId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Credential</Form.Label>
							<select
								{...props}
								bind:value={$repositoryFields.connectionId}
								class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
							>
								<option value="">Choose a connection</option>
								{#each connections as connection (connection.id)}
									<option value={connection.id}>
										{providerLabel(connection.provider)} · {connectionLabel(connection)}
									</option>
								{/each}
							</select>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field form={repositoryForm} name="fullName">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Repository</Form.Label>
							{#if chosenConnection?.authKind === "app"}
								<select
									{...props}
									bind:value={$repositoryFields.fullName}
									disabled={offering || offered.length === 0}
									class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
								>
									<option value="">
										{#if offering}
											Reading what the installation reaches…
										{:else if offerUnreadable}
											GitHub could not be asked what this installation reaches
										{:else if offered.length === 0}
											This installation was granted no repositories
										{:else}
											Choose a repository
										{/if}
									</option>
									{#each offered as available (available.externalId)}
										<option value={available.fullName}>{available.fullName}</option>
									{/each}
								</select>
							{:else}
								<Input {...props} bind:value={$repositoryFields.fullName} placeholder="acme/api" />
							{/if}
						{/snippet}
					</Form.Control>
					<Form.Description>
						{#if chosenConnection?.authKind === "app" && offerUnreadable}
							The connection may have stopped working. Verify it, then try again.
						{:else if chosenConnection?.authKind === "app"}
							Only what you granted the installation is listed. Grant more on GitHub and it
							appears here.
						{:else}
							Write it the way the platform does, as owner/name.
						{/if}
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field form={repositoryForm} name="mirrorLabel">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Label to watch for</Form.Label>
							<Input {...props} bind:value={$repositoryFields.mirrorLabel} placeholder="norn" />
						{/snippet}
					</Form.Control>
					<Form.Description>
						A platform issue carrying this label is brought across and kept in step. Everything
						else stays where it is.
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				<div class="flex flex-wrap gap-2">
					<Button type="submit" disabled={$adding}>
						{$addingDelayed ? "Reading the repository…" : "Connect it"}
					</Button>
				</div>
			</form>
		{/if}
	{/if}
</div>
