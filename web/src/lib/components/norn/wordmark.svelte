<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLSpanElement>, HTMLSpanElement> = $props();
</script>

<span
	bind:this={ref}
	class={cn("inline-flex items-center gap-2 text-ink-900", className)}
	{...restProps}
>
	<svg viewBox="0 0 21 24" width="11.4" height="13" fill="currentColor" aria-hidden="true">
		<path d="M8.8 0H12.2V24H8.8Z" />
		<path d="M2.404 1.5L7.9 6.996V11.804L0 3.904Z" />
		<path d="M13.1 12.196L21 20.096L18.596 22.5L13.1 17.004Z" />
	</svg>
	<span class="font-mono text-lg leading-none font-medium tracking-wordmark uppercase">Norn</span>
</span>
