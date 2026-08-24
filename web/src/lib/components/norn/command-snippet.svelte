<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import Copy from "@lucide/svelte/icons/copy";
	import { Button } from "$lib/components/ui/button/index.js";
	import { cn } from "$lib/utils.js";

	let {
		command,
		label = "Copy",
		copied = false,
		variant = "secondary",
		class: className,
		oncopy,
	}: {
		command: string;
		label?: string;
		copied?: boolean;
		variant?: "secondary" | "ghost";
		class?: string;
		oncopy: (command: string) => void;
	} = $props();
</script>

<div class={cn("flex min-w-0 flex-col gap-2", className)}>
	<p class="min-w-0 rounded-md bg-paper-2 p-3 font-mono text-xs break-all text-ink-900">
		{command}
	</p>
	<Button {variant} size="sm" class="w-max" onclick={() => oncopy(command)}>
		{#if copied}
			<Check aria-hidden="true" />
			Copied
		{:else}
			<Copy aria-hidden="true" />
			{label}
		{/if}
	</Button>
</div>
