<script lang="ts">
	import { standingLabels, type ProjectStanding } from "./projects";
	import { cn } from "$lib/utils.js";

	let { standing, class: className }: { standing: ProjectStanding; class?: string } = $props();

	const tones: Record<ProjectStanding, string> = {
		on_track: "border-success/40 text-success",
		at_risk: "border-warning/40 text-warning",
		completed: "border-success/40 text-success",
		cancelled: "border-line-strong text-muted-foreground",
	};
</script>

<span
	class={cn(
		"inline-flex shrink-0 items-center rounded-xs border px-1.5 py-0.5 font-mono text-2xs tracking-caps uppercase",
		tones[standing],
		className
	)}
>
	{standingLabels[standing]}
</span>
