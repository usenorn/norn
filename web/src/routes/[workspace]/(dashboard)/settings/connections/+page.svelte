<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Plug from "@lucide/svelte/icons/plug";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDate } from "$lib/time";
	import {
		capabilityLabel,
		type WorkspaceConnectionListing,
		type WorkspaceMCPConnection,
	} from "$lib/account/connections";
	import { workspaceConnectionsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? workspaceConnectionsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let replaced = $state<WorkspaceConnectionListing | null>(null);
	let revoking = $state<string | null>(null);
	let failed = $state(false);

	const listing = $derived<WorkspaceConnectionListing>(
		replaced ?? preview?.listing ?? data.listing
	);
	const busy = $derived(preview?.busy || revoking !== null);
	const current = $derived(listing.kind === "ready" ? listing.connections : []);

	async function revoke(connectionId: string) {
		revoking = connectionId;
		failed = false;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/mcp-connections/{connectionId}",
				{ params: { path: { workspaceId: data.workspace.id, connectionId } } }
			);

			if (error) {
				failed = true;

				return;
			}

			const remaining = current.filter((owned) => owned.connection.id !== connectionId);

			replaced =
				remaining.length === 0 ? { kind: "empty" } : { kind: "ready", connections: remaining };
		} catch {
			failed = true;
		} finally {
			revoking = null;
		}
	}

	function formatted(instant: string | undefined): string | null {
		if (!instant) return null;

		return onDate(instant, "UTC");
	}

	function reachLabel(owned: WorkspaceMCPConnection): string {
		return owned.connection.followsMembership
			? "Follows the owner's membership"
			: "Narrowed by its owner";
	}
</script>

<svelte:head><title>AI clients · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">AI clients</h1>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failed}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>Check your connection and try again.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if listing.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>Administrators only</Alert.Title>
					<Alert.Description>
						Ask a workspace administrator if a connection needs revoking.
					</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load connections</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading connections…</p>
			{:else}
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">
						Connections reaching {data.workspace.name}
					</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Every AI client able to act here, and whose authority it acts under. Revoking cuts a
						connection off everywhere it reaches, from its next request.
					</p>
				</div>

				{#if current.length === 0}
					<p class="text-sm text-muted-foreground">No AI clients can reach this workspace.</p>
				{:else}
					<ul class="rounded-lg border border-line-subtle bg-paper-0">
						{#each current as owned (owned.connection.id)}
							<li
								class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
							>
								<div class="flex min-w-0 flex-col gap-1.5">
									<div class="flex items-center gap-2">
										<Plug class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
										<span class="truncate text-sm text-ink-900">
											{owned.connection.clientName}
										</span>
										<Tag name={capabilityLabel[owned.connection.capability]} />
									</div>
									<p class="text-xs text-muted-foreground">
										Acts for {owned.ownerName}
										<span class="font-mono">{owned.ownerEmail}</span>
									</p>
									<p class="text-xs text-muted-foreground">
										{reachLabel(owned)}
										&middot; Connected {formatted(owned.connection.createdAt)}
										{#if owned.connection.lastUsedAt}&middot; Last used {formatted(
												owned.connection.lastUsedAt
											)}{/if}
									</p>
								</div>
								<div class="flex-none">
									<Button
										variant="secondary"
										size="sm"
										disabled={busy}
										onclick={() => revoke(owned.connection.id)}
									>
										{revoking === owned.connection.id ? "Revoking…" : "Revoke"}
									</Button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
		</div>
	</div>
</div>
