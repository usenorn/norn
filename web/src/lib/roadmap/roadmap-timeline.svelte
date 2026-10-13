<script lang="ts">
	import { timelinePlacement, type RoadmapTimeline, type TimelineWindow } from "./public-roadmap";

	let { timeline, window }: { timeline: RoadmapTimeline | undefined; window: TimelineWindow } =
		$props();

	const placement = $derived(timelinePlacement(timeline, window));
</script>

{#if placement}
	<div class="relative h-1.5 w-full rounded-xs bg-paper-3" aria-hidden="true">
		{#if placement.kind === "span"}
			<div
				class="absolute inset-y-0 min-w-1.5 rounded-xs bg-status-active"
				style="left: {placement.offset}%; width: {placement.width}%"
			></div>
		{:else}
			<div
				class="absolute inset-y-0 w-1.5 -translate-x-1/2 rounded-full bg-status-active"
				style="left: {placement.offset}%"
			></div>
		{/if}
	</div>
{/if}
