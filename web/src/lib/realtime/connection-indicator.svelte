<script lang="ts">
	import { cn } from "$lib/utils.js";
	import type { RealtimeState } from "./connection.svelte";

	let { state, class: className }: { state: RealtimeState; class?: string } = $props();

	const labels: Record<RealtimeState, string> = {
		connecting: "Connecting",
		live: "Live",
		reconnecting: "Reconnecting",
		stale: "Not receiving updates",
		off: "Updates are off",
	};

	const dots: Record<RealtimeState, string> = {
		connecting: "bg-muted-foreground",
		live: "bg-status-active",
		reconnecting: "bg-status-active/40",
		stale: "bg-destructive",
		off: "bg-muted-foreground/40",
	};
</script>

<span
	class={cn("flex items-center gap-1.5 text-2xs text-muted-foreground", className)}
	title={labels[state]}
>
	<span
		class={cn("size-1.5 shrink-0 rounded-full", dots[state], state === "reconnecting" && "animate-pulse")}
		aria-hidden="true"
	></span>
	<span class="sr-only" role="status" aria-live="polite">{labels[state]}</span>
	<span aria-hidden="true">{labels[state]}</span>
</span>
