<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { lift } from "$lib/motion";

	let {
		message,
		action = "Undo",
		onaction,
		class: className,
	}: {
		message: string;
		action?: string;
		onaction?: () => void;
		class?: string;
	} = $props();
</script>

<div
	role="status"
	aria-live="polite"
	transition:lift
	class={cn(
		"notch-ink flex items-center gap-2.5 px-3 py-2 text-md [--keycap-lip:var(--ink-700)]",
		className
	)}
>
	<span class="whitespace-nowrap">{message}</span>
	{#if onaction}
		<button
			type="button"
			onclick={onaction}
			class="motion-control cursor-pointer border-b border-transparent px-0.5 font-mono text-2xs font-medium tracking-caps uppercase hover:border-primary-foreground"
		>
			{action}
		</button>
	{/if}
</div>
