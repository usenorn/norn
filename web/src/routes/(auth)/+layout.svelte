<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Wordmark from "$lib/components/norn/wordmark.svelte";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();
</script>

<div class="flex min-h-dvh flex-col bg-background">
	<header class="flex flex-none flex-wrap items-center gap-2 px-5 pt-5 sm:px-6">
		<a href="/sign-in" class="inline-flex" aria-label="Norn">
			<Wordmark />
		</a>
		{#if data.auth.selfHosted}
			<Eyebrow>Self-hosted</Eyebrow>
		{/if}
	</header>

	<main
		class="flex flex-1 flex-col items-center px-5 pt-8 pb-[calc(--spacing(16)+env(safe-area-inset-bottom))] sm:pb-[calc(--spacing(24)+env(safe-area-inset-bottom))]"
	>
		{@render children()}
	</main>
</div>
