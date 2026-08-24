<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { machineStanding, standingLabels, standingTones, type Runner } from "./runners";

	let { machine, class: className }: { machine: Runner; class?: string } = $props();

	const tone = {
		waiting: "text-status-not-started",
		working: "text-status-active",
		attention: "text-amber-700 dark:text-amber-400",
		bad: "text-danger",
	};

	const pip = {
		waiting: "bg-status-not-started",
		working: "bg-status-active animate-breathe",
		attention: "bg-amber-700 dark:bg-amber-400",
		bad: "bg-danger",
	};

	const standing = $derived(machineStanding(machine));
</script>

<span
	class={cn(
		"inline-flex min-w-0 items-center gap-1.5 text-sm",
		tone[standingTones[standing]],
		className
	)}
>
	<span
		aria-hidden="true"
		class={cn("size-1.5 flex-none rounded-full", pip[standingTones[standing]])}
	></span>
	<span class="truncate">{standingLabels[standing]}</span>
</span>
