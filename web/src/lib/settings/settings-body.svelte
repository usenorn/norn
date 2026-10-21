<script module lang="ts">
	import { tv, type VariantProps } from "tailwind-variants";

	export const settingsWidth = tv({
		base: "w-full",
		variants: {
			width: {
				compact: "max-w-160",
				standard: "max-w-224",
				wide: "max-w-none",
			},
		},
		defaultVariants: {
			width: "standard",
		},
	});

	export type SettingsWidth = VariantProps<typeof settingsWidth>["width"];
</script>

<script lang="ts">
	import type { Snippet } from "svelte";

	let { width = "standard", children }: { width?: SettingsWidth; children: Snippet } = $props();
</script>

<div class="flex-1 overflow-auto">
	<div
		class="mx-auto w-full max-w-272 px-4 pt-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))] sm:px-6 lg:px-8"
	>
		<div class={settingsWidth({ width, class: "flex flex-col gap-8" })}>
			{@render children()}
		</div>
	</div>
</div>
