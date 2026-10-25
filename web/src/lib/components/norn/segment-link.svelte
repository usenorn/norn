<script lang="ts" module>
	import { tv } from "tailwind-variants";

	export const segmentLinkVariants = tv({
		base: "inline-flex h-8 flex-none cursor-pointer items-center gap-1.5 rounded-xs px-2 caption motion-control sm:h-5.5",
		variants: {
			current: {
				true: "bg-primary text-primary-foreground",
				false: "text-ink-600 hover:bg-accent",
			},
		},
		defaultVariants: {
			current: false,
		},
	});
</script>

<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "#lib/utils.js";

	let {
		href,
		current = false,
		class: className,
		children,
	}: { href: string; current?: boolean; class?: string; children: Snippet } = $props();
</script>

<a {href} aria-current={current ? "page" : undefined} class={cn(segmentLinkVariants({ current }), className)}>
	{@render children()}
</a>
