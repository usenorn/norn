<script lang="ts">
	import { page } from "$app/state";
	import BadgeCheck from "@lucide/svelte/icons/badge-check";
	import Check from "@lucide/svelte/icons/check";
	import Minus from "@lucide/svelte/icons/minus";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { onDate } from "$lib/time";
	import {
		featureLabel,
		freeForever,
		statusLabel,
		statusNote,
		type LicenceView,
	} from "$lib/licence/licence";
	import { licencePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? licencePreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	const view = $derived<LicenceView>(preview?.view ?? data.view);
	const report = $derived(view.kind === "ready" ? view.report : null);

	function on(instant: string | undefined): string {
		if (!instant) return "—";

		return onDate(instant, "UTC");
	}
</script>

<svelte:head><title>Licence · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-2 border-b border-line-subtle px-4">
		<BadgeCheck class="size-4 text-muted-foreground" aria-hidden="true" />
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Licence</h1>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not read the licence</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if report}
				<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-5">
					<div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							{statusLabel(report.status)}
						</h2>
						{#if report.holder}
							<span class="text-sm text-muted-foreground">{report.holder}</span>
						{/if}
					</div>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{statusNote(report.status)}
					</p>

					{#if report.status !== "absent"}
						<dl class="flex flex-col gap-1 text-sm">
							{#if report.issuedAt}
								<div class="flex flex-wrap gap-x-2">
									<dt class="text-muted-foreground">Issued</dt>
									<dd class="text-ink-900">{on(report.issuedAt)}</dd>
								</div>
							{/if}
							<div class="flex flex-wrap gap-x-2">
								<dt class="text-muted-foreground">
									{report.status === "active" ? "Expires" : "Expired"}
								</dt>
								<dd class="text-ink-900">{on(report.expiresAt)}</dd>
							</div>
							{#if report.status === "grace"}
								<div class="flex flex-wrap gap-x-2">
									<dt class="text-muted-foreground">Features stop</dt>
									<dd class="text-ink-900">{on(report.graceEndsAt)}</dd>
								</div>
							{/if}
						</dl>
					{/if}
				</section>

				<section class="flex flex-col gap-3">
					<h2 class="text-md font-medium tracking-snug text-ink-900">What a licence covers</h2>
					<ul class="flex flex-col gap-2">
						{#each report.features as feature (feature.name)}
							<li
								class="flex items-center gap-2.5 rounded-lg border border-line-default bg-paper-0 px-4 py-3 text-sm"
							>
								{#if feature.enabled}
									<Check class="size-4 text-success" aria-hidden="true" />
								{:else}
									<Minus class="size-4 text-muted-foreground" aria-hidden="true" />
								{/if}
								<span class="text-ink-900">{featureLabel(feature.name)}</span>
								<span class="ml-auto text-xs text-muted-foreground">
									{feature.enabled ? "Available" : "Not available"}
								</span>
							</li>
						{/each}
					</ul>
				</section>

				<section class="flex flex-col gap-3">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Free forever</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						These are never gated, on any tier, including a self-hosted instance with no licence
						at all. Norn never counts the people, issues or work in an instance for pricing.
					</p>
					<ul class="flex flex-col gap-2">
						{#each freeForever as promise (promise)}
							<li class="flex items-start gap-2.5 text-sm text-muted-foreground">
								<Check class="mt-0.5 size-4 shrink-0 text-success" aria-hidden="true" />
								<span class="text-pretty">{promise}</span>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		</div>
	</div>
</div>
