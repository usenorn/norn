<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import GithubAppPanel from "$lib/source-control/github-app-panel.svelte";
	import VerdictBanner from "$lib/source-control/verdict-banner.svelte";
	import {
		brokenLabel,
		connectionLabel,
		detailOf,
		failureMessage,
		providerLabel,
		routingLabel,
		sourceControlConnectPath,
		sourceControlConnectionPath,
		sourceControlFailure,
		sourceControlIdentitiesPath,
		sourceControlRepositoryPath,
		sourceControlVerdict,
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

	const connections = $derived(view.kind === "list" ? view.connections : []);
	const githubConnection = $derived(
		connections.find((connection) => connection.provider === "github") ?? null,
	);
	const repositories = $derived(view.kind === "list" ? view.repositories : []);
	const repositoriesUnavailable = $derived(
		view.kind === "list" ? Boolean(view.repositoriesUnavailable) : false,
	);

	const shown = $derived(failure ?? preview?.failure);
	const shownApplication = $derived(preview?.application ?? data.application);
	const shownNotice = $derived(preview?.notice ?? data.notice);

	const verdict = $derived(
		sourceControlVerdict({
			slug: workspace.slug,
			application: shownApplication,
			connections,
			repositories,
		}),
	);

	function deliversCentrally(connectionId: string): boolean {
		return connections.some(
			(connection) => connection.id === connectionId && connection.authKind === "app",
		);
	}

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

			loaded = { kind: "list", connections: [created, ...connections], repositories };
		},
	});

	const {
		form: connectFields,
		enhance: connectEnhance,
		submitting: connecting,
		delayed: connectDelayed,
	} = connectForm;
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
		<VerdictBanner {verdict} />

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
			connectedTo={githubConnection ? connectionLabel(githubConnection) : null}
			repositoryCount={repositories.length}
			connectHref={sourceControlConnectPath(workspace.slug)}
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
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">Repositories</h2>
				{#if connections.length > 0 && repositories.length > 0}
					<Button variant="secondary" href={sourceControlConnectPath(workspace.slug)}>
						Connect a repository
					</Button>
				{/if}
			</div>

			{#if repositoriesUnavailable}
				<Alert.Root variant="destructive">
					<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
					<Alert.Title>The connected repositories could not be read</Alert.Title>
					<Alert.Description>
						This is not the same as having none. Until this loads, what Norn watches is unknown —
						check your connection and reload.
					</Alert.Description>
				</Alert.Root>
			{:else if repositories.length === 0}
				<div
					class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
				>
					<GitBranch class="size-5 text-muted-foreground" aria-hidden="true" />
					<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
						Norn is watching nothing yet, so every event GitHub sends is discarded. Connect a
						repository and its branches, commits and pull requests start reaching their issues.
					</p>
					{#if connections.length > 0}
						<Button href={sourceControlConnectPath(workspace.slug)} class="mt-1">
							Connect a repository
						</Button>
					{/if}
				</div>
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
									Watching for “{repository.mirrorLabel}”. {routingLabel(repository)}.
								</p>
								{#if !repository.hookInstalled && !deliversCentrally(repository.connectionId)}
									<p class="flex items-center gap-1.5 text-sm text-warning">
										<TriangleAlert class="size-icon-row shrink-0" aria-hidden="true" />
										The webhook is not installed yet — the sweep is carrying it.
									</p>
								{/if}
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

		<details class="rounded-lg border border-line-subtle p-4">
			<summary
				class="cursor-pointer text-md font-medium tracking-snug text-ink-900 marker:text-muted-foreground"
			>
				Connect without the application
			</summary>

			<form method="POST" use:connectEnhance class="mt-4 flex flex-col gap-4">
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
		</details>
	{/if}
</div>