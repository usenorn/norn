<script lang="ts">
	import { AlertDialog as AlertDialogPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: AlertDialogPrimitive.OverlayProps = $props();
</script>

<AlertDialogPrimitive.Overlay
	bind:ref
	data-slot="alert-dialog-overlay"
	class={cn("bg-surface-scrim supports-backdrop-filter:backdrop-blur-[6px] data-open:animate-scrim data-closed:animate-dismiss fixed inset-0 isolate z-50", className)}
	{...restProps}
/>
