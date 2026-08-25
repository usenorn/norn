<script lang="ts">
	import { Dialog as SheetPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: SheetPrimitive.OverlayProps = $props();
</script>

<SheetPrimitive.Overlay
	bind:ref
	data-slot="sheet-overlay"
	class={cn(
		"fixed inset-0 isolate z-50 bg-surface-scrim supports-backdrop-filter:backdrop-blur-[6px] data-open:animate-scrim data-closed:animate-dismiss",
		className
	)}
	{...restProps}
/>
