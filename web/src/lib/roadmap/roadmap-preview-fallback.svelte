<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Map from "@lucide/svelte/icons/map";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Retry from "$lib/components/norn/retry.svelte";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";

	let {
		view,
		workspaceName,
		previewHref,
	}: {
		view: { kind: "loading" } | { kind: "forbidden" } | { kind: "not_found" } | { kind: "unavailable" };
		workspaceName: string;
		previewHref: string;
	} = $props();
</script>

{#if view.kind === "loading"}
	<div class="flex flex-col gap-8" role="status" aria-busy="true">
		<span class="sr-only">Loading the draft preview</span>
		<div class="flex flex-col gap-2">
			<Skeleton class="h-7 w-2/3" />
			<Skeleton class="h-4 w-full" />
		</div>
		<div class="flex flex-col gap-px">
			{#each [0, 1, 2] as row (row)}
				<Skeleton class="h-20 w-full" />
			{/each}
		</div>
	</div>
{:else if view.kind === "forbidden"}
	<Alert.Root variant="muted" class="my-auto">
		<div class="col-span-full flex flex-col gap-0.5">
			<Eyebrow>No access · admin only</Eyebrow>
			<Alert.Title>
				<h1>Only administrators preview the public roadmap</h1>
			</Alert.Title>
			<Alert.Description>
				What {workspaceName} shares publicly is decided by its administrators. Ask one of them to change it.
			</Alert.Description>
		</div>
	</Alert.Root>
{:else if view.kind === "not_found"}
	<Empty.Root class="my-auto">
		<Empty.Media variant="icon"><Map aria-hidden="true" /></Empty.Media>
		<Empty.Header>
			<Empty.Title>
				<h1>This entry is not in the draft</h1>
			</Empty.Title>
			<Empty.Description>
				It was removed from the roadmap, or its project or issue is no longer within reach, so publishing would not
				show it.
			</Empty.Description>
		</Empty.Header>
		<Empty.Content>
			<Button size="sm" variant="secondary" href={previewHref}>Back to the preview</Button>
		</Empty.Content>
	</Empty.Root>
{:else}
	<div class="my-auto flex flex-col items-start gap-3">
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>
				<h1>We could not render the draft preview</h1>
			</Alert.Title>
			<Alert.Description>Wait a moment and try again.</Alert.Description>
		</Alert.Root>
		<Retry />
	</div>
{/if}
