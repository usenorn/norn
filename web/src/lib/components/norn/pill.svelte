<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const pillVariants = tv({
		base: "inline-flex shrink-0 items-center gap-1 rounded-xs border px-1.5 py-0.5 text-xs font-medium [&_svg]:size-3 [&_svg]:shrink-0",
		variants: {
			tone: {
				success: "border-success/40 text-success",
				warning: "border-warning/40 text-warning",
				active: "border-status-active/40 text-status-active",
				muted: "border-line-strong text-muted-foreground",
			},
		},
		defaultVariants: {
			tone: "muted",
		},
	});

	export type PillTone = VariantProps<typeof pillVariants>["tone"];
</script>

<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils.js";

	let {
		tone = "muted",
		class: className,
		children,
	}: { tone?: PillTone; class?: string; children: Snippet } = $props();
</script>

<span class={cn(pillVariants({ tone }), className)}>
	{@render children()}
</span>
