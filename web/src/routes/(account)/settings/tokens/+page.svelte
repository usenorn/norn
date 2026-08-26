<script lang="ts">
	import { page } from "$app/state";
	import { invalidate } from "$app/navigation";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { onDate } from "$lib/time";
	import { mcpAddCommand, mcpServerUrl } from "$lib/agents/agents";
	import { mintTokenSchema, type GrantInput } from "$lib/account/mint-token-schema";
	import {
		daysUntil,
		defaultExpiryDays,
		expiringSoonDays,
		failureMessage,
		scopeGroups,
		scopeLabels,
		writes,
		type APIScope,
		type APIToken,
		type MintOutcome,
		type TokenFailure,
		type TokenListing,
	} from "$lib/account/tokens";
	import { tokensPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "mint-token-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? tokensPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let copiedValue = $state<string | null>(null);
	let revoking = $state<string | null>(null);
	let revealed = $state<Extract<MintOutcome, { kind: "minted" }> | null>(null);

	const listing = $derived<TokenListing>(preview?.listing ?? data.listing);
	const workspaces = $derived(preview?.workspaces ?? data.workspaces);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		dataType: "json",
		validators: zod4Client(mintTokenSchema),
		resetForm: false,
	});
	const { form: formData, enhance, submitting, message } = form;

	$effect(() => {
		if ($formData.expiresInDays === 0) {
			formData.update((entered) => ({ ...entered, expiresInDays: defaultExpiryDays }));
		}
	});

	const outcome = $derived($message ?? null);

	const failure = $derived<TokenFailure | null>(
		outcome && outcome.kind !== "minted" ? outcome : null
	);

	$effect(() => {
		if (outcome?.kind === "minted") revealed = outcome;
	});

	const minted = $derived(revealed ?? (listing.kind === "minted" ? listing : null));

	const serverUrl = $derived(mcpServerUrl(page.url.origin));
	const addCommand = $derived(
		minted ? mcpAddCommand(minted.token.name, page.url.origin, minted.value) : ""
	);

	const busy = $derived(preview?.busy || $submitting || revoking !== null);

	const current = $derived(
		listing.kind === "ready" || listing.kind === "minted" ? listing.tokens : []
	);

	const showForm = $derived(listing.kind !== "forbidden" && listing.kind !== "unavailable");

	function grantFor(workspaceId: string): GrantInput | undefined {
		return $formData.grants.find((grant) => grant.workspaceId === workspaceId);
	}

	function toggleWorkspace(workspaceId: string, checked: boolean) {
		formData.update((entered) => ({
			...entered,
			grants: checked
				? [...entered.grants, { workspaceId, allTeams: true, teamIds: [] }]
				: entered.grants.filter((grant) => grant.workspaceId !== workspaceId),
		}));
	}

	function setAllTeams(workspaceId: string, allTeams: boolean) {
		formData.update((entered) => ({
			...entered,
			grants: entered.grants.map((grant) =>
				grant.workspaceId === workspaceId ? { ...grant, allTeams, teamIds: [] } : grant
			),
		}));
	}

	function toggleTeam(workspaceId: string, teamId: string, checked: boolean) {
		formData.update((entered) => ({
			...entered,
			grants: entered.grants.map((grant) => {
				if (grant.workspaceId !== workspaceId) return grant;

				const teamIds = checked
					? [...grant.teamIds, teamId]
					: grant.teamIds.filter((id) => id !== teamId);

				return { ...grant, teamIds };
			}),
		}));
	}

	function toggleScope(scope: APIScope, checked: boolean) {
		const next = new Set($formData.scopes);

		if (checked) {
			next.add(scope);
		} else {
			next.delete(scope);
		}

		formData.update((entered) => ({ ...entered, scopes: [...next] }));
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
		copiedValue = text;
	}

	async function revoke(tokenId: string) {
		revoking = tokenId;

		try {
			const { error } = await api.DELETE("/tokens/{tokenId}", {
				params: { path: { tokenId } },
			});

			if (error) {
				message.set(error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" });

				return;
			}

			await invalidate(keys.page(page.route.id));
		} catch {
			message.set({ kind: "unavailable" });
		} finally {
			revoking = null;
		}
	}

	function formatted(instant: string | undefined): string | null {
		if (!instant) return null;

		return onDate(instant, "UTC");
	}

	function reaches(token: APIToken): string {
		return token.grants
			.map((grant) => {
				const workspace = workspaces.find((candidate) => candidate.id === grant.workspaceId);
				const name = workspace?.name ?? "a workspace you have left";

				if (grant.allTeams) return name;

				return `${name} (${grant.teamIds?.length ?? 0} team${grant.teamIds?.length === 1 ? "" : "s"})`;
			})
			.join(" · ");
	}

	function expiringSoon(token: APIToken): number | null {
		const days = daysUntil(token.expiresAt, data.now);

		return days !== null && days <= expiringSoonDays ? days : null;
	}
</script>

<svelte:head><title>API tokens · Norn</title></svelte:head>

<SettingsPage
	title="API tokens"
	description="Personal credentials for scripts, CLIs and MCP clients."
	Icon={KeyRound}
	meta={listing.kind === "loading" ? "loading" : `${current.length} active`}
	width="compact"
>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if listing.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage tokens</Alert.Title>
					<Alert.Description>Ask an administrator if you need one.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load your tokens</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if minted}
				<section class="flex flex-col gap-3 rounded-lg border border-success/40 bg-paper-0 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							Copy {minted.token.name} now
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							This is the only time the token is shown. If you lose it, revoke it and mint another.
						</p>
					</div>
					<p class="rounded-md bg-paper-100 p-3 font-mono text-xs break-all text-ink-900">
						{minted.value}
					</p>
					<div>
						<Button variant="secondary" size="sm" onclick={() => copy(minted.value)}>
							{copiedValue === minted.value ? "Copied" : "Copy token"}
						</Button>
					</div>

					<div class="flex flex-col gap-2 border-t border-line-subtle pt-3">
						<h3 class="text-sm font-medium tracking-snug text-ink-900">Connect an AI client</h3>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							A token reaches Norn's MCP server as you, in the workspaces and teams you granted it.
							To have an AI client act under its own name, with its own limits and approvals,
							register an agent in a workspace instead.
						</p>
						<p class="rounded-md bg-paper-100 p-3 font-mono text-xs break-all text-ink-900">
							{addCommand}
						</p>
						<div class="flex flex-wrap gap-2">
							<Button variant="secondary" size="sm" onclick={() => copy(addCommand)}>
								{copiedValue === addCommand ? "Copied" : "Copy command"}
							</Button>
							<Button variant="ghost" size="sm" onclick={() => copy(serverUrl)}>
								{copiedValue === serverUrl ? "Copied" : "Copy server URL"}
							</Button>
						</div>
					</div>
				</section>
			{/if}

			{#if showForm}
				<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Mint a token</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							A token acts for you and can never do more than you can. If your access changes, the
							token's changes with it.
						</p>
					</div>

					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Name</Form.Label>
								<Input
									{...props}
									bind:value={$formData.name}
									disabled={busy}
									placeholder="CI pipeline"
									autocomplete="off"
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="grants">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Where it reaches</Form.Label>
								<div {...props} class="flex flex-col gap-3">
									{#each workspaces as workspace (workspace.id)}
										{@const grant = grantFor(workspace.id)}
										<div class="rounded-md border border-line-subtle p-3">
											<div class="flex items-start gap-2">
												<Checkbox
													id={`workspace-${workspace.id}`}
													disabled={busy}
													checked={grant !== undefined}
													onCheckedChange={(checked) =>
														toggleWorkspace(workspace.id, checked === true)}
												/>
												<label
													for={`workspace-${workspace.id}`}
													class="text-sm leading-normal text-ink-900"
												>
													{workspace.name}
												</label>
											</div>

											{#if grant}
												<div class="mt-3 flex flex-col gap-2 pl-6">
													<div class="flex items-center gap-2">
														<Checkbox
															id={`all-teams-${workspace.id}`}
															disabled={busy}
															checked={grant.allTeams}
															onCheckedChange={(checked) =>
																setAllTeams(workspace.id, checked === true)}
														/>
														<label
															for={`all-teams-${workspace.id}`}
															class="text-sm leading-normal text-muted-foreground"
														>
															Every team you can see
														</label>
													</div>

													{#if !grant.allTeams}
														{@const teams = data.teams[workspace.id] ?? []}
														{#if teams.length === 0}
															<p class="text-xs text-muted-foreground">
																No teams you can see in this workspace.
															</p>
														{:else}
															<div class="flex flex-col gap-1.5">
																{#each teams as team (team.id)}
																	<div class="flex items-center gap-2">
																		<Checkbox
																			id={`team-${workspace.id}-${team.id}`}
																			disabled={busy}
																			checked={grant.teamIds.includes(team.id)}
																			onCheckedChange={(checked) =>
																				toggleTeam(workspace.id, team.id, checked === true)}
																		/>
																		<label
																			for={`team-${workspace.id}-${team.id}`}
																			class="text-sm leading-normal text-ink-600"
																		>
																			{team.name}
																		</label>
																	</div>
																{/each}
															</div>
														{/if}
													{/if}
												</div>
											{/if}
										</div>
									{/each}
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="scopes">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Permissions</Form.Label>
								<div {...props} class="flex flex-col gap-3">
									{#each scopeGroups as group (group.resource)}
										<div class="flex flex-col gap-1.5">
											<p class="text-xs font-medium tracking-snug text-ink-900">{group.title}</p>
											{#each group.scopes as scope (scope)}
												<div class="flex items-start gap-2">
													<Checkbox
														id={`scope-${scope}`}
														disabled={busy}
														checked={$formData.scopes.includes(scope)}
														onCheckedChange={(checked) => toggleScope(scope, checked === true)}
													/>
													<label
														for={`scope-${scope}`}
														class="text-sm leading-normal text-pretty text-ink-600"
													>
														{scopeLabels[scope] ?? scope}
														<span class="font-mono text-xs text-muted-foreground">{scope}</span>
														{#if writes(scope)}
															<span class="text-xs text-muted-foreground">· admins only</span>
														{/if}
													</label>
												</div>
											{/each}
										</div>
									{/each}
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="expiresInDays">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Expires after</Form.Label>
								<Input
									{...props}
									type="number"
									min="1"
									max="365"
									disabled={busy}
									bind:value={$formData.expiresInDays}
								/>
								<Form.Description>
									Days. A token cannot outlive a year, and cannot be extended.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<div>
						<Form.Button disabled={busy}>
							{$submitting ? "Minting…" : "Mint token"}
						</Form.Button>
					</div>
				</form>
			{/if}

			{#if listing.kind === "loading"}
				<div class="flex flex-col gap-2" aria-busy="true" aria-label="Loading API tokens">
					<Skeleton class="h-14 w-full" />
					<Skeleton class="h-14 w-full" />
				</div>
			{:else if current.length === 0 && showForm}
				<Empty.Root>
					<Empty.Media variant="icon"><KeyRound aria-hidden="true" /></Empty.Media>
					<Empty.Header>
						<Empty.Title>No live tokens</Empty.Title>
						<Empty.Description>Mint one above when a tool needs access to your account.</Empty.Description>
					</Empty.Header>
				</Empty.Root>
			{:else if current.length > 0}
				<section class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Your tokens</h2>
					<ul class="rounded-lg border border-line-subtle bg-paper-0">
						{#each current as token (token.id)}
							{@const soon = expiringSoon(token)}
							<li
								class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
							>
								<div class="flex min-w-0 flex-col gap-1.5">
									<div class="flex items-center gap-2">
										<KeyRound class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
										<span class="truncate text-sm text-ink-900">{token.name}</span>
									</div>
									<div class="flex flex-wrap gap-1">
										{#each token.scopes as scope (scope)}
											<Tag name={scope} />
										{/each}
									</div>
									<p class="text-xs text-muted-foreground">Reaches {reaches(token)}</p>
									<p class="text-xs text-muted-foreground">
										Created {formatted(token.createdAt)}
										{#if token.lastUsedAt}&middot; Last used {formatted(token.lastUsedAt)}{/if}
										{#if token.expiresAt}&middot; Expires {formatted(token.expiresAt)}{/if}
									</p>
									{#if soon !== null}
										<p class="text-xs text-destructive">
											{soon <= 0
												? "Expired"
												: `Expires in ${soon} day${soon === 1 ? "" : "s"} — mint a replacement`}
										</p>
									{/if}
								</div>
								<div class="flex-none">
									<Button
										variant="secondary"
										size="sm"
										disabled={busy}
										onclick={() => revoke(token.id)}
									>
										{revoking === token.id ? "Revoking…" : "Revoke"}
									</Button>
								</div>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
</SettingsPage>
