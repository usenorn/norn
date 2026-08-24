<script lang="ts">
	import { page } from "$app/state";
	import { invalidateAll } from "$app/navigation";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";

	const title = $derived(page.status === 404 ? "Not found" : "Something went wrong");
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="relative flex min-h-0 flex-1 flex-col overflow-auto">
	<div class="my-auto flex flex-col items-center gap-4 px-6 py-12 text-center">
		<span
			class="flex size-8 items-center justify-center border-b-2 border-line-strong text-ink-300"
		>
			<TriangleAlert class="size-5" aria-hidden="true" />
		</span>

		<div class="flex max-w-form flex-col items-center gap-1.5">
			<Eyebrow>Error {page.status}</Eyebrow>
			<h1 class="text-lg font-medium tracking-snug text-ink-900">{title}</h1>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				{page.error?.message}
			</p>
		</div>

		<Button variant="outline" size="sm" onclick={() => invalidateAll()}>Try again</Button>

		{#if page.error?.reference}
			<p class="font-mono text-xs break-all text-muted-foreground">
				Reference {page.error.reference}
			</p>
		{/if}
	</div>
</div>
