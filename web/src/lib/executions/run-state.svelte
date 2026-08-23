<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { stateLabel, stateTone, type ExecutionState } from "./executions";

	let { state, class: className }: { state: ExecutionState; class?: string } = $props();

	const tone = {
		waiting: "text-status-not-started",
		working: "text-status-active",
		attention: "text-amber-700 dark:text-amber-400",
		done: "text-status-complete",
		bad: "text-danger",
	};

	const pip = {
		waiting: "bg-status-not-started",
		working: "bg-status-active animate-breathe",
		attention: "bg-amber-700 dark:bg-amber-400",
		done: "bg-status-complete",
		bad: "bg-danger",
	};

	const shown = $derived(stateTone(state));
</script>

<span class={cn("inline-flex items-center gap-1.5 text-sm", tone[shown], className)}>
	<span aria-hidden="true" class={cn("size-1.5 flex-none rounded-full", pip[shown])}></span>
	{stateLabel(state)}
</span>
