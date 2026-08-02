<script lang="ts">
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import Funnel from "@lucide/svelte/icons/funnel";
	import Plus from "@lucide/svelte/icons/plus";
	import Settings from "@lucide/svelte/icons/settings";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import TaskRow from "$lib/components/norn/task-row.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { workspacePath } from "$lib/workspace/navigation";
	import { myTasksPreviewStates, type TaskBucket } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? (myTasksPreviewStates[page.url.searchParams.get("state") ?? "default"] ??
					myTasksPreviewStates.default)
			: undefined
	);

	const buckets = $derived<TaskBucket[]>(preview?.buckets ?? []);
	const total = $derived(buckets.reduce((sum, bucket) => sum + bucket.tasks.length, 0));

	let cursor = $state(0);
	const flat = $derived(buckets.flatMap((bucket) => bucket.tasks));

	function onkeydown(event: KeyboardEvent) {
		if (event.metaKey || event.ctrlKey || event.altKey) return;
		const target = event.target as HTMLElement | null;
		if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)))
			return;
		if (event.key === "j" || event.key === "ArrowDown") {
			event.preventDefault();
			cursor = Math.min(cursor + 1, flat.length - 1);
		} else if (event.key === "k" || event.key === "ArrowUp") {
			event.preventDefault();
			cursor = Math.max(cursor - 1, 0);
		}
	}
</script>

<svelte:head><title>My tasks · Norn</title></svelte:head>
<svelte:window {onkeydown} />

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<CircleDot class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">My tasks</h1>
				<span class="font-mono text-xs text-muted-foreground tabular-nums">{total}</span>
			</div>
			<Button variant="outline" size="icon-sm" aria-label="Notifications">
				<Bell class="size-icon-toolbar" aria-hidden="true" />
			</Button>
			<Button size="sm">
				<Plus aria-hidden="true" />
				New task
			</Button>
		</div>
		<div class="flex h-8.5 items-center gap-2.5 overflow-hidden border-t border-line-subtle pr-3 pl-3.5">
			<Button variant="ghost" size="sm">
				<Funnel aria-hidden="true" />
				Filter
			</Button>
			<div class="min-w-2 flex-1"></div>
			<span class="text-sm whitespace-nowrap text-muted-foreground">Grouped by due date</span>
			<Button variant="ghost" size="icon-sm" aria-label="Display options">
				<Settings class="size-icon-toolbar" aria-hidden="true" />
			</Button>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		{#if buckets.length === 0}
			<div class="flex flex-col items-center gap-1.5 px-6 py-12 text-center">
				<span
					class="mb-1.5 flex size-8 items-center justify-center border-b-2 border-line-strong text-ink-300"
				>
					<CircleDot class="size-icon-toolbar" aria-hidden="true" />
				</span>
				<p class="text-md font-medium tracking-snug text-ink-900">Nothing assigned to you</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Work assigned to you lands here, grouped by when it is due.
				</p>
			</div>
		{:else}
			{#each buckets as bucket (bucket.key)}
				{@const offset = flat.indexOf(bucket.tasks[0])}
				<section>
					<div
						class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-3.5"
					>
						<span
							class="font-mono text-2xs font-medium tracking-eyebrow uppercase {bucket.emphasis
								? 'text-ink-900'
								: 'text-ink-600'}"
						>
							{bucket.label}
						</span>
						<span class="font-mono text-2xs text-muted-foreground tabular-nums">
							{bucket.tasks.length}
						</span>
						<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
					</div>
					{#each bucket.tasks as task, index (task.id)}
						<TaskRow {task} href={workspacePath(data.workspace.slug, `/issues/${task.id}`)} cursor={offset + index === cursor} />
					{/each}
				</section>
			{/each}
		{/if}
	</div>

	<div
		class="hidden h-7.5 flex-none items-center justify-end gap-4 border-t border-line-subtle bg-card px-3.5 md:flex"
	>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="C" />new task
		</span>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="⌘ K" />go anywhere
		</span>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="↑ ↓" />move
		</span>
	</div>
</div>
