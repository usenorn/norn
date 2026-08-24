<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Network from "@lucide/svelte/icons/network";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { api } from "$lib/api";
	import {
		absentPolicies,
		changeLabel,
		detailPairs,
		failureMessage,
		operationLabel,
		unknownPolicies,
		type DirectoryAbsentPolicy,
		type DirectoryChange,
		type DirectoryFailure,
		type DirectoryUnknownPolicy,
		type DirectoryView,
	} from "$lib/directory/directory";
	import { onDateAndTime } from "$lib/time";
	import { directoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? directoryPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let loaded = $state<DirectoryView | null>(null);
	let localFailure = $state<DirectoryFailure | null>(null);
	let working = $state(false);
	let expanded = $state<string | null>(null);
	let changes = $state<Record<string, DirectoryChange[]>>({});

	const workspace = $derived(page.data.workspace);
	const view = $derived<DirectoryView>(loaded ?? preview?.view ?? data.view);
	const busy = $derived(preview?.busy ?? working);
	const failure = $derived(preview?.failure ?? localFailure);
	const live = $derived(view.kind === "connected" || view.kind === "issued" ? view : null);
	const origin = $derived(page.url.origin);

	function classify(status: number): DirectoryFailure {
		if (status === 503) return { kind: "unlicensed" };
		if (status === 403) return { kind: "forbidden" };

		return { kind: "unavailable" };
	}

	async function connect() {
		working = true;
		localFailure = null;

		const { data: settings, error, response } = await api.POST(
			"/workspaces/{workspaceId}/directory",
			{ params: { path: { workspaceId: workspace.id } } }
		);

		working = false;

		if (error || !settings?.connection) {
			localFailure = classify(response.status);

			return;
		}

		loaded = {
			kind: "issued",
			connection: settings.connection,
			scimBaseUrl: settings.scimBaseUrl,
			runs: live?.runs ?? [],
			token: settings.token ?? "",
		};
	}

	async function rotate() {
		working = true;
		localFailure = null;

		const { data: settings, error, response } = await api.POST(
			"/workspaces/{workspaceId}/directory/token",
			{ params: { path: { workspaceId: workspace.id } } }
		);

		working = false;

		if (error || !settings?.connection) {
			localFailure = classify(response.status);

			return;
		}

		loaded = {
			kind: "issued",
			connection: settings.connection,
			scimBaseUrl: settings.scimBaseUrl,
			runs: live?.runs ?? [],
			token: settings.token ?? "",
		};
	}

	async function configure(change: Partial<{
		enabled: boolean;
		onUnknown: DirectoryUnknownPolicy;
		onAbsent: DirectoryAbsentPolicy;
		adminGroup: string;
	}>) {
		if (!live) return;

		working = true;
		localFailure = null;

		const { data: settings, error, response } = await api.PATCH(
			"/workspaces/{workspaceId}/directory",
			{
				params: { path: { workspaceId: workspace.id } },
				body: {
					enabled: change.enabled ?? live.connection.enabled,
					onUnknown: change.onUnknown ?? live.connection.onUnknown,
					onAbsent: change.onAbsent ?? live.connection.onAbsent,
					adminGroup: change.adminGroup ?? live.connection.adminGroup,
				},
			}
		);

		working = false;

		if (error || !settings?.connection) {
			localFailure = classify(response.status);

			return;
		}

		loaded = {
			kind: "connected",
			connection: settings.connection,
			scimBaseUrl: settings.scimBaseUrl,
			runs: live.runs,
		};
	}

	async function disconnect() {
		working = true;
		localFailure = null;

		const { error, response } = await api.DELETE("/workspaces/{workspaceId}/directory", {
			params: { path: { workspaceId: workspace.id } },
		});

		working = false;

		if (error) {
			localFailure = classify(response.status);

			return;
		}

		loaded = { kind: "disconnected" };
	}

	async function inspect(runId: string) {
		if (expanded === runId) {
			expanded = null;

			return;
		}

		expanded = runId;

		if (changes[runId]) return;

		const { data: found } = await api.GET(
			"/workspaces/{workspaceId}/directory/runs/{runId}/changes",
			{ params: { path: { workspaceId: workspace.id, runId } } }
		);

		changes = { ...changes, [runId]: found ?? [] };
	}
</script>

<svelte:head><title>Directory · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-2 border-b border-line-subtle px-4">
		<Network class="size-4 text-muted-foreground" aria-hidden="true" />
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Directory</h1>
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "unlicensed"}
				<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							Directory synchronization is not part of this instance's licence
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Signing in through your identity provider stays free and keeps working. Having that
							provider create, update and remove members automatically needs a licence that covers
							it.
						</p>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Provisioning is never charged per person, and members it already created keep their
						places.
						<a class="underline underline-offset-2 hover:text-ink-900" href="/settings/licence">
							See what this instance is licensed for
						</a>.
					</p>
				</section>
			{:else if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage provisioning here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the directory settings</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "disconnected"}
				<section class="flex flex-col gap-4 rounded-lg border border-line-default bg-paper-0 p-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							Let your identity provider manage who is here
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn speaks SCIM 2.0. Connecting issues a credential you paste into your provider;
							from then on it creates, updates and removes members, and its groups drive team
							membership.
						</p>
					</div>
					<div>
						<Button onclick={connect} disabled={busy}>
							{busy ? "Connecting…" : "Connect a directory"}
						</Button>
					</div>
				</section>
			{:else if live}
				{#if view.kind === "issued"}
					<section class="flex flex-col gap-3 rounded-lg border border-success/40 bg-paper-0 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								Copy this credential now
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								This is the only time it is shown. If you lose it, replace it below.
							</p>
						</div>
						<code
							class="overflow-x-auto rounded-md border border-line-subtle bg-paper-1 px-3 py-2 font-mono text-xs text-ink-900"
						>{view.token}</code>
					</section>
				{/if}

				<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-4">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Point your provider here</h2>
					<div class="flex flex-col gap-1.5">
						<Label for="scim-url">SCIM base URL</Label>
						<Input id="scim-url" readonly value={`${origin}${live.scimBaseUrl}`} />
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{#if live.connection.lastSyncAt}
							Last change from your provider {onDateAndTime(
								live.connection.lastSyncAt,
								workspace.timezone
							)}.
						{:else}
							Nothing has arrived from your provider yet.
						{/if}
					</p>
					<div class="flex flex-wrap gap-2">
						<Button variant="secondary" size="sm" onclick={rotate} disabled={busy}>
							Replace the credential
						</Button>
						<Button variant="ghost" size="sm" onclick={disconnect} disabled={busy}>
							Disconnect
						</Button>
					</div>
				</section>

				<section class="flex flex-col gap-4 rounded-lg border border-line-default bg-paper-0 p-4">
					<h2 class="text-md font-medium tracking-snug text-ink-900">How changes are handled</h2>

					<div class="flex flex-col gap-1.5">
						<Label for="on-unknown">Somebody arrives who is not here yet</Label>
						<Select.Root
							type="single"
							value={live.connection.onUnknown}
							onValueChange={(value) => configure({ onUnknown: value as DirectoryUnknownPolicy })}
							disabled={busy}
						>
							<Select.Trigger id="on-unknown">
								{unknownPolicies.find((p) => p.value === live.connection.onUnknown)?.label}
							</Select.Trigger>
							<Select.Content>
								{#each unknownPolicies as policy (policy.value)}
									<Select.Item value={policy.value} label={policy.label}>{policy.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						<p class="text-xs leading-normal text-muted-foreground text-pretty">
							{unknownPolicies.find((p) => p.value === live.connection.onUnknown)?.note}
						</p>
					</div>

					<div class="flex flex-col gap-1.5">
						<Label for="on-absent">Somebody is removed upstream</Label>
						<Select.Root
							type="single"
							value={live.connection.onAbsent}
							onValueChange={(value) => configure({ onAbsent: value as DirectoryAbsentPolicy })}
							disabled={busy}
						>
							<Select.Trigger id="on-absent">
								{absentPolicies.find((p) => p.value === live.connection.onAbsent)?.label}
							</Select.Trigger>
							<Select.Content>
								{#each absentPolicies as policy (policy.value)}
									<Select.Item value={policy.value} label={policy.label}>{policy.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						<p class="text-xs leading-normal text-muted-foreground text-pretty">
							{absentPolicies.find((p) => p.value === live.connection.onAbsent)?.note}
						</p>
					</div>

					<div class="flex flex-col gap-1.5">
						<Label for="admin-group">Group whose members administer this workspace</Label>
						<Input
							id="admin-group"
							value={live.connection.adminGroup}
							placeholder="norn-admins"
							disabled={busy}
							onchange={(event) => configure({ adminGroup: event.currentTarget.value })}
						/>
						<p class="text-xs leading-normal text-muted-foreground text-pretty">
							Everybody else the directory sends is an ordinary member. Leave this empty and the
							directory grants nobody administration. Sync will never remove the last
							administrator.
						</p>
					</div>
				</section>

				<section class="flex flex-col gap-3">
					<h2 class="text-md font-medium tracking-snug text-ink-900">What the directory changed</h2>

					{#if live.runs.length === 0}
						<div
							class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
						>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Nothing yet. Every change your provider makes is recorded here, so membership never
								moves without a trace.
							</p>
						</div>
					{:else}
						<ul class="flex flex-col gap-2">
							{#each live.runs as run (run.id)}
								<li class="rounded-lg border border-line-default bg-paper-0">
									<button
										type="button"
										class="flex w-full flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 text-left"
										onclick={() => inspect(run.id)}
										aria-expanded={expanded === run.id}
									>
										<span class="text-sm text-ink-900">{operationLabel(run.operation)}</span>
										<span
											class="rounded px-1.5 py-0.5 text-xs {run.outcome === 'succeeded'
												? 'bg-success/15 text-success'
												: run.outcome === 'refused'
													? 'bg-status-blocked/15 text-status-blocked'
													: 'bg-destructive/15 text-destructive'}"
										>
											{run.outcome}
										</span>
										<span class="ml-auto text-xs text-muted-foreground">
											{onDateAndTime(run.startedAt, workspace.timezone)}
										</span>
										{#if run.detail}
											<span class="flex-[1_1_100%] text-xs text-muted-foreground">{run.detail}</span>
										{/if}
									</button>

									{#if expanded === run.id}
										<div class="border-t border-line-subtle px-4 py-3">
											{#if (changes[run.id] ?? []).length === 0}
												<p class="text-xs text-muted-foreground">
													This run changed nothing in the workspace.
												</p>
											{:else}
												<ul class="flex flex-col gap-2">
													{#each changes[run.id] as change (change.id)}
														<li class="flex flex-wrap items-baseline gap-x-2 text-sm">
															<span class="text-ink-900">{changeLabel(change.kind)}</span>
															<span class="text-muted-foreground">{change.subject}</span>
															{#each detailPairs(change) as [key, value] (key)}
																<span class="text-xs text-muted-foreground">{key}: {value}</span>
															{/each}
														</li>
													{/each}
												</ul>
											{/if}
										</div>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			{/if}
		</div>
	</div>
</div>
