<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const kbdVariants = tv({
		base: "keycap inline-flex h-4.5 min-w-4.5 items-center justify-center gap-1 rounded-xs border px-1 font-mono text-2xs font-medium",
		variants: {
			tone: {
				default: "border-line-strong bg-paper-2 text-ink-600",
				inverse:
					"border-line-inverse bg-transparent text-paper-0/74 [--keycap-lip:rgba(15,17,22,0.14)]",
			},
		},
		defaultVariants: { tone: "default" },
	});

	export type KbdTone = VariantProps<typeof kbdVariants>["tone"];
</script>

<script lang="ts">
	import { cn } from "$lib/utils.js";

	let {
		keys,
		tone = "default",
		class: className,
	}: { keys: string; tone?: KbdTone; class?: string } = $props();
</script>

<kbd class={cn(kbdVariants({ tone }), className)}>
	{keys}
</kbd>
