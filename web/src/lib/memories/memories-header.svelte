<script lang="ts">
	import type { Snippet } from "svelte";
	import Brain from "@lucide/svelte/icons/brain";
	import { memoriesPath } from "./memories";

	let {
		workspace,
		title,
		actions,
	}: {
		workspace: string;
		title?: string;
		actions?: Snippet;
	} = $props();

	const crumb =
		"relative min-w-0 truncate text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring pointer-coarse:after:absolute pointer-coarse:after:-inset-3";
</script>

<div class="flex-none border-b border-line-default">
	<div class="flex h-11 min-w-0 items-center gap-2 pr-3 pl-4">
		<Brain class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
		<nav aria-label="Breadcrumbs" class="flex min-w-0 flex-1 items-center gap-2">
			{#if title === undefined}
				<h1 class="text-md font-semibold tracking-snug text-ink-900">Memories</h1>
			{:else}
				<a href={memoriesPath(workspace)} class="{crumb} shrink-0">Memories</a>
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<h1 class="min-w-0 truncate text-md font-semibold tracking-snug text-ink-900">{title}</h1>
			{/if}
		</nav>
		{#if actions}
			<span class="ml-auto flex shrink-0 items-center gap-1.5">
				{@render actions()}
			</span>
		{/if}
	</div>
</div>
