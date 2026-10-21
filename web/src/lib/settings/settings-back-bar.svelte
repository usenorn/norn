<script lang="ts">
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import type { Snippet } from "svelte";

	let { href, label, trailing }: { href: string; label: string; trailing?: Snippet } = $props();
</script>

<div class="flex-none border-b border-line-subtle">
	<div class="mx-auto flex h-11 w-full max-w-272 items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
		<a
			{href}
			class="-ml-1 flex min-h-8 items-center gap-1.5 rounded-sm px-1 text-sm text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>{label}</span>
		</a>
		{@render trailing?.()}
	</div>
</div>
