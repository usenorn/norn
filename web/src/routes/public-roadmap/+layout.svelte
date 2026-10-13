<script lang="ts">
	import Wordmark from "$lib/components/norn/wordmark.svelte";
	import type { LayoutProps } from "./$types";

	let { children }: LayoutProps = $props();
</script>

<div class="flex min-h-dvh flex-col bg-background">
	<main class="mx-auto flex w-full max-w-160 flex-1 flex-col px-4 pt-10 pb-12 sm:px-6 sm:pt-16">
		{@render children()}
	</main>

	<footer
		class="flex flex-none items-center justify-center gap-2 px-4 pt-4 pb-[calc(--spacing(6)+env(safe-area-inset-bottom))] text-xs text-muted-foreground"
	>
		<span>Published with</span>
		<Wordmark />
	</footer>
</div>
