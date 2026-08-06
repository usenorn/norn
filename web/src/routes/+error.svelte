<script lang="ts">
	import { page } from "$app/state";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Wordmark from "$lib/components/norn/wordmark.svelte";

	const title = $derived(page.status === 404 ? "Not found" : "Something went wrong");
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="flex min-h-dvh flex-col bg-background">
	<header class="flex flex-none flex-wrap items-center gap-2 px-6 pt-5">
		<a href="/" class="inline-flex" aria-label="Norn">
			<Wordmark />
		</a>
	</header>

	<main
		class="flex flex-1 flex-col items-center px-5 pt-8 pb-[calc(--spacing(16)+env(safe-area-inset-bottom))] sm:pb-[calc(--spacing(24)+env(safe-area-inset-bottom))]"
	>
		<div class="my-auto flex w-full flex-col items-center gap-4">
			<div class="notch w-full max-w-form">
				<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
					<div class="flex flex-col gap-1.5">
						<Eyebrow>Error {page.status}</Eyebrow>
						<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
						<p class="text-md leading-normal text-muted-foreground text-pretty">
							{page.error?.message}
						</p>
					</div>

					<Button href="/">Back to Norn</Button>
				</div>
			</div>

			{#if page.error?.reference}
				<p class="text-center font-mono text-xs break-all text-muted-foreground">
					Reference {page.error.reference}
				</p>
			{/if}
		</div>
	</main>
</div>
