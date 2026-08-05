<script lang="ts">
	import { page } from "$app/state";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Plug from "@lucide/svelte/icons/plug";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { capabilityLabel, capabilityLede, type ConsentState } from "$lib/account/connections";
	import { authorizePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? authorizePreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let deciding = $state<"approve" | "deny" | null>(null);
	let failed = $state(false);

	const consent = $derived<ConsentState>(
		failed ? { kind: "failed" } : (preview?.consent ?? data.consent)
	);
	const busy = $derived(preview?.busy ?? deciding);

	async function decide(action: "approve" | "deny") {
		deciding = action;
		failed = false;

		const path =
			action === "approve"
				? ("/mcp/authorizations/{requestId}/approve" as const)
				: ("/mcp/authorizations/{requestId}/deny" as const);

		try {
			const { data: decision, error } = await api.POST(path, {
				params: { path: { requestId: data.requestId } },
			});

			if (error || !decision) {
				failed = true;

				return;
			}

			window.location.href = decision.redirectTo;
		} catch {
			failed = true;
		} finally {
			deciding = null;
		}
	}
</script>

<svelte:head><title>Connect an AI client · Norn</title></svelte:head>

<div class="flex flex-1 flex-col items-center px-4 py-6">
	<div class="my-auto flex w-full max-w-form flex-col gap-4">
		{#if consent.kind === "loading"}
			<p class="text-center text-sm text-muted-foreground">Loading the request…</p>
		{:else if consent.kind === "expired"}
			<Alert.Root variant="muted">
				<CircleDashed aria-hidden="true" />
				<Alert.Title>This request has expired or was already decided</Alert.Title>
				<Alert.Description>
					Go back to the app that asked and connect again. Nothing was granted.
				</Alert.Description>
			</Alert.Root>
		{:else if consent.kind === "failed"}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>Couldn't reach the server</Alert.Title>
				<Alert.Description>
					Check your connection and try again. Nothing was granted.
				</Alert.Description>
			</Alert.Root>
		{:else}
			{@const request = consent.request}
			<div class="notch flex w-full flex-col gap-4 p-5 sm:p-6">
				<div class="flex flex-col gap-1.5">
					<div class="flex items-center gap-2">
						<Plug class="size-5 text-muted-foreground" aria-hidden="true" />
						<h1 class="text-2xl font-medium tracking-title text-ink-900">
							Connect {request.clientName}
						</h1>
					</div>
					<p class="text-md leading-normal text-muted-foreground text-pretty">
						{request.clientName} wants to connect to your Norn as {capabilityLabel[
							request.capability
						].toLowerCase()}.
					</p>
				</div>

				<div class="flex flex-col gap-3 rounded-md border border-line-subtle p-3">
					<p class="text-sm leading-normal text-ink-600 text-pretty">
						{capabilityLede[request.capability]}
					</p>
					<div class="flex flex-col gap-1.5">
						<p class="text-sm leading-normal text-ink-600 text-pretty">
							One connection reaches every workspace you belong to — today that is:
						</p>
						<ul class="flex flex-col gap-1 pl-1">
							{#each request.workspaces as workspace (workspace.id)}
								<li class="text-sm text-ink-900">{workspace.name}</li>
							{/each}
						</ul>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Workspaces you join later are included automatically. It can never see or do more
							than you can, and narrows the moment your access does.
						</p>
					</div>
				</div>

				<div class="flex flex-col gap-2 sm:flex-row-reverse">
					<Button class="sm:flex-1" disabled={busy !== null} onclick={() => decide("approve")}>
						{busy === "approve" ? "Connecting…" : "Allow access"}
					</Button>
					<Button
						variant="secondary"
						class="sm:flex-1"
						disabled={busy !== null}
						onclick={() => decide("deny")}
					>
						{busy === "deny" ? "Refusing…" : "Deny"}
					</Button>
				</div>
			</div>

			<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
				You can narrow or revoke this connection at any time in
				<a href="/settings/connections" class="text-link hover:text-link-hover hover:underline">
					AI clients
				</a>.
			</p>
		{/if}
	</div>
</div>
