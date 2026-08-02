<script lang="ts">
	import Copy from "@lucide/svelte/icons/copy";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { cn } from "$lib/utils.js";
	import type { Diagnostic } from "$lib/auth/types";

	let {
		label,
		entries,
		keyWidth = "w-20 sm:w-24",
		class: className,
	}: {
		label: string;
		entries: Diagnostic[];
		keyWidth?: string;
		class?: string;
	} = $props();
</script>

<div
	class={cn("overflow-hidden rounded-lg border border-line-strong bg-paper-0", className)}
>
	<div class="flex h-7 items-center justify-between gap-2 border-b border-line-subtle pr-1 pl-2">
		<Eyebrow>{label}</Eyebrow>
		<Button variant="ghost" size="sm">
			<Copy aria-hidden="true" />
			Copy
		</Button>
	</div>
	<dl class="flex flex-col gap-0.5 p-2">
		{#each entries as entry (entry.key)}
			<div class="flex gap-2 font-mono text-xs leading-normal">
				<dt class="flex-none text-muted-foreground {keyWidth}">{entry.key}</dt>
				<dd class="min-w-0 flex-1 break-all text-ink-600">{entry.value}</dd>
			</div>
		{/each}
	</dl>
</div>
