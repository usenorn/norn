<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Map from "@lucide/svelte/icons/map";
	import Retry from "#lib/components/norn/retry.svelte";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Empty from "#lib/components/ui/empty/index.js";
	import PublicRoadmapHead from "./public-roadmap-head.svelte";

	let {
		view,
		roadmapHref,
	}: {
		view: { kind: "loading" } | { kind: "unavailable" } | { kind: "failed"; reference: string };
		roadmapHref?: string;
	} = $props();
</script>

{#if view.kind === "loading"}
	<PublicRoadmapHead title="Roadmap" />
	<div class="flex flex-col gap-8" role="status" aria-busy="true">
		<span class="sr-only">Loading the roadmap</span>
		<div class="flex flex-col gap-2">
			<div class="h-7 w-2/3 animate-breathe rounded-md bg-paper-2"></div>
			<div class="h-4 w-full animate-breathe rounded-md bg-paper-2"></div>
		</div>
		<ul class="flex flex-col gap-px">
			{#each [0, 1, 2] as row (row)}
				<li class="h-20 animate-breathe rounded-md bg-paper-2"></li>
			{/each}
		</ul>
	</div>
{:else if view.kind === "unavailable"}
	<PublicRoadmapHead title="Roadmap unavailable" />
	<Empty.Root class="my-auto">
		<Empty.Media variant="icon"><Map aria-hidden="true" /></Empty.Media>
		<Empty.Header>
			<Empty.Title>
				<h1>This roadmap is not available</h1>
			</Empty.Title>
			<Empty.Description>
				The link may be mistyped, or what it pointed to is no longer shared.
			</Empty.Description>
		</Empty.Header>
		{#if roadmapHref}
			<Empty.Content>
				<Button size="sm" variant="secondary" href={roadmapHref}>Back to the roadmap</Button>
			</Empty.Content>
		{/if}
	</Empty.Root>
{:else}
	<PublicRoadmapHead title="Something went wrong" />
	<div class="my-auto flex flex-col items-start gap-3">
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>
				<h1>We could not load this roadmap</h1>
			</Alert.Title>
			<Alert.Description>Wait a moment and try again.</Alert.Description>
		</Alert.Root>
		<Retry />
		{#if view.reference}
			<p class="font-mono text-xs break-all text-muted-foreground">Reference {view.reference}</p>
		{/if}
	</div>
{/if}
