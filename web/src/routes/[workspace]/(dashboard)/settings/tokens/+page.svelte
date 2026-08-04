<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Copy from "@lucide/svelte/icons/copy";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDate } from "$lib/time";
	import { mintTokenSchema } from "$lib/workspace/mint-token-schema";
	import {
		failureMessage,
		mintFailure,
		scopeCatalog,
		scopeLabels,
		type TokenFailure,
		type TokenListing,
	} from "$lib/workspace/tokens";
	import { tokensPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? tokensPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let submitted = $state<TokenListing | null>(null);
	let failure = $state<TokenFailure | null>(null);
	let copied = $state(false);
	let revoking = $state<string | null>(null);

	const workspace = $derived(data.workspace);
	const listing = $derived<TokenListing>(submitted ?? preview?.listing ?? data.listing);

	const formId = "mint-token-form";

	const form = superForm(defaults(zod4(mintTokenSchema)), {
		SPA: true,
		validators: zod4Client(mintTokenSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;
			copied = false;

			try {
				const { data: minted, error } = await api.POST("/workspaces/{workspaceId}/tokens", {
					params: { path: { workspaceId: workspace.id } },
					body: { name: entered.data.name, scopes: entered.data.scopes },
				});

				if (error) {
					const mapped = mintFailure(error);

					if (mapped.kind === "name_taken") {
						setError(entered, "name", failureMessage(mapped));

						return;
					}

					failure = mapped;

					return;
				}

				if (!minted) {
					failure = { kind: "unavailable" };

					return;
				}

				submitted = {
					kind: "minted",
					token: minted.token,
					value: minted.value,
					tokens: [minted.token, ...current],
				};

				formData.set({ name: "", scopes: [] });
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const busy = $derived(preview?.busy || $submitting || revoking !== null);

	const current = $derived(
		listing.kind === "ready" || listing.kind === "minted" ? listing.tokens : []
	);

	const minted = $derived(listing.kind === "minted" ? listing : null);
	const showForm = $derived(listing.kind !== "forbidden" && listing.kind !== "unavailable");

	function toggleScope(scope: string, checked: boolean) {
		const next = new Set($formData.scopes);

		if (checked) {
			next.add(scope);
		} else {
			next.delete(scope);
		}

		formData.update((entered) => ({ ...entered, scopes: [...next] }));
	}

	async function copyValue() {
		if (!minted) return;
		await navigator.clipboard.writeText(minted.value);
		copied = true;
	}

	async function revoke(tokenId: string) {
		revoking = tokenId;
		failure = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/tokens/{tokenId}", {
				params: { path: { workspaceId: workspace.id, tokenId } },
			});

			if (error) {
				failure = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			const remaining = current.filter((token) => token.id !== tokenId);

			submitted = remaining.length === 0 ? { kind: "empty" } : { kind: "ready", tokens: remaining };
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			revoking = null;
		}
	}

	function formatted(instant: string | undefined): string | null {
		if (!instant) return null;

		return onDate(instant, data.workspace.timezone);
	}
</script>

<svelte:head><title>API tokens · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">API tokens</h1>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
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
					<Alert.Title>You may not manage tokens here</Alert.Title>
					<Alert.Description>
						Ask an administrator of {workspace.name} if you need one.
					</Alert.Description>
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

					<p
						class="rounded-lg border border-line-strong bg-paper-1 px-3 py-2.5 font-mono text-xs break-all text-ink-600"
					>
						{minted.value}
					</p>

					<div class="flex flex-wrap gap-2">
						<Button variant="secondary" size="sm" onclick={copyValue}>
							<Copy aria-hidden="true" />
							{copied ? "Copied" : "Copy token"}
						</Button>
					</div>
				</section>
			{/if}

			{#if showForm}
				<section class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Mint a token</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							A token acts for you in {workspace.name} and can never do more than you can.
						</p>
					</div>

					<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
						<Form.Field {form} name="name">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Name</Form.Label>
									<Input
										{...props}
										placeholder="CI pipeline"
										disabled={busy}
										bind:value={$formData.name}
									/>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="scopes">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Permissions</Form.Label>
									<div {...props} class="flex flex-col gap-2">
										{#each scopeCatalog as scope (scope)}
											<div class="flex items-start gap-2">
												<Checkbox
													id={`scope-${scope}`}
													disabled={busy}
													checked={$formData.scopes.includes(scope)}
													onCheckedChange={(checked) => toggleScope(scope, checked === true)}
												/>
												<label
													for={`scope-${scope}`}
													class="text-sm leading-normal text-ink-600 text-pretty"
												>
													{scopeLabels[scope] ?? scope}
													<span class="font-mono text-xs text-muted-foreground">{scope}</span>
												</label>
											</div>
										{/each}
									</div>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<div>
							<Button type="submit" form={formId} disabled={busy}>
								{busy ? "Minting" : "Mint token"}
							</Button>
						</div>
					</form>
				</section>
			{/if}

			{#if listing.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading your tokens…</p>
			{:else if current.length === 0 && showForm}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					You have no live tokens in {workspace.name}.
				</p>
			{:else if current.length > 0}
				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium tracking-snug text-ink-900">Live tokens</h2>
					<ul class="flex flex-col rounded-lg border border-line-subtle">
						{#each current as token (token.id)}
							<li
								class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
							>
								<div class="flex min-w-0 flex-col gap-1">
									<div class="flex items-center gap-2">
										<KeyRound class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
										<span class="truncate text-sm text-ink-900">{token.name}</span>
									</div>
									<div class="flex flex-wrap gap-1">
										{#each token.scopes as scope (scope)}
											<Tag name={scope} />
										{/each}
									</div>
									<p class="text-xs text-muted-foreground">
										Created {formatted(token.createdAt)}
										{#if token.lastUsedAt}· Last used {formatted(token.lastUsedAt)}{/if}
										{#if token.expiresAt}· Expires {formatted(token.expiresAt)}{/if}
									</p>
								</div>

								<Button
									variant="ghost"
									size="sm"
									disabled={busy}
									onclick={() => revoke(token.id)}
								>
									{revoking === token.id ? "Revoking" : "Revoke"}
								</Button>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		</div>
	</div>
</div>
