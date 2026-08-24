<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import RunRow from "$lib/executions/run-row.svelte";
	import { noReviewsLine, waitingCount, type ReviewQueue } from "$lib/executions/reviews";
	import { workspacePath } from "$lib/workspace/navigation";
	import { reviewPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? reviewPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const queue = $derived<ReviewQueue>(preview?.queue ?? data.queue);
</script>

<svelte:head>
	<title>Reviews · {workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<h1 class="text-sm text-ink-900">Reviews</h1>
			{#if queue.kind === "ready" && queue.runs.length > 0}
				<span class="text-xs text-muted-foreground">{waitingCount(queue.runs)}</span>
			{/if}
		</div>
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-4 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if queue.kind === "loading"}
				<p class="my-auto text-sm text-muted-foreground">Reading what is waiting…</p>
			{:else if queue.kind === "unavailable"}
				<div class="my-auto">
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>We could not load the review queue</Alert.Title>
						<Alert.Description>
							Something went wrong and nothing changed. Wait a moment and try again.
						</Alert.Description>
					</Alert.Root>
				</div>
			{:else if queue.runs.length === 0}
				<p class="my-auto max-w-prose text-sm text-muted-foreground text-pretty">
					{noReviewsLine}
				</p>
			{:else}
				<section class="flex min-w-0 flex-col gap-2" aria-label="Runs waiting for review">
					<Eyebrow rule>Waiting for you</Eyebrow>
					<div class="flex min-w-0 flex-col">
						{#each queue.runs as waiting (waiting.execution.id)}
							<RunRow
								execution={waiting.execution}
								change={waiting.change}
								href={workspacePath(workspace.slug, `/executions/${waiting.execution.id}`)}
								timezone={workspace.timezone}
							/>
						{/each}
					</div>
				</section>
			{/if}
		</div>
	</div>
</div>
