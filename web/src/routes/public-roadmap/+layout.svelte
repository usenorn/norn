<script lang="ts">
	import { navigating, page } from "$app/state";
	import Wordmark from "$lib/components/norn/wordmark.svelte";
	import PublicRoadmapFallback from "$lib/roadmap/public-roadmap-fallback.svelte";
	import { isPublicRoadmapRoute } from "$lib/roadmap/host";
	import type { LayoutProps } from "./$types";

	let { children }: LayoutProps = $props();

	const loading = $derived(
		navigating.to !== null &&
			isPublicRoadmapRoute(navigating.to.route.id) &&
			navigating.to.url.pathname !== page.url.pathname
	);
</script>

<div class="flex min-h-dvh flex-col bg-background">
	<main class="mx-auto flex w-full max-w-160 flex-1 flex-col px-4 pt-10 pb-12 sm:px-6 sm:pt-16">
		{#if loading}
			<PublicRoadmapFallback view={{ kind: "loading" }} />
		{:else}
			{@render children()}
		{/if}
	</main>

	<footer
		class="flex flex-none items-center justify-center gap-2 px-4 pt-4 pb-[calc(--spacing(6)+env(safe-area-inset-bottom))] text-xs text-muted-foreground"
	>
		<span>Published with</span>
		<Wordmark />
	</footer>
</div>
