<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { lift } from "$lib/motion";

	let {
		message,
		href,
		action = "Undo",
		onaction,
		onnavigate,
		class: className,
	}: {
		message: string;
		href?: string;
		action?: string;
		onaction?: () => void;
		onnavigate?: () => void;
		class?: string;
	} = $props();
</script>

<div
	transition:lift
	class={cn(
		"notch-ink flex max-w-full items-center gap-2.5 px-3 py-2 text-md [--keycap-lip:var(--ink-700)]",
		className
	)}
>
	{#if href}
		<a
			{href}
			onclick={onnavigate}
			class="min-w-0 flex-1 underline-offset-2 text-pretty hover:underline"
		>
			{message}
		</a>
	{:else}
		<span class="min-w-0 flex-1 text-pretty">{message}</span>
	{/if}
	{#if onaction}
		<button
			type="button"
			onclick={onaction}
			class="motion-control shrink-0 cursor-pointer border-b border-transparent px-0.5 font-mono text-2xs font-medium tracking-caps uppercase hover:border-primary-foreground"
		>
			{action}
		</button>
	{/if}
</div>
