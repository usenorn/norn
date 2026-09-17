<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		rule = false,
		tone = "muted",
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLSpanElement>, HTMLSpanElement> & {
		rule?: boolean;
		tone?: "muted" | "danger";
	} = $props();
</script>

<span
	bind:this={ref}
	class={cn(
		"font-mono text-2xs tracking-eyebrow uppercase",
		tone === "danger" ? "text-destructive" : "text-muted-foreground",
		rule &&
			"flex items-center gap-2 whitespace-nowrap after:h-px after:flex-1 after:bg-line-default after:content-['']",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</span>
