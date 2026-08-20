<script lang="ts">
	import { onDestroy } from "svelte";
	import Toast from "$lib/components/norn/toast.svelte";
	import { useToasts } from "./toasts.svelte";

	const toasts = useToasts();

	onDestroy(() => toasts.stop());
</script>

<div
	role="status"
	aria-live="polite"
	class="pointer-events-none fixed right-4 bottom-[calc(--spacing(19)+env(safe-area-inset-bottom))] left-4 z-70 flex justify-end md:bottom-4 md:left-auto"
	onpointerenter={() => toasts.hold()}
	onpointerleave={() => toasts.release()}
	onfocusin={() => toasts.hold()}
	onfocusout={() => toasts.release()}
>
	{#if toasts.shown}
		<Toast
			message={toasts.shown.message}
			href={toasts.shown.href}
			action={toasts.shown.action}
			onaction={toasts.shown.onaction}
			onnavigate={() => toasts.dismiss()}
			class="pointer-events-auto"
		/>
	{/if}
</div>
