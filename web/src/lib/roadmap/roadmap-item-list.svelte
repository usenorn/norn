<script lang="ts">
	import RoadmapItemList from "./roadmap-item-list.svelte";
	import RoadmapItemRow from "./roadmap-item-row.svelte";
	import type { PublicRoadmapItem, TimelineWindow } from "./public-roadmap";

	let {
		items,
		entryHref,
		window,
		nested = false,
		label,
	}: {
		items: PublicRoadmapItem[];
		entryHref: (id: string) => string;
		window?: TimelineWindow;
		nested?: boolean;
		label?: string;
	} = $props();
</script>

<ul
	aria-label={label}
	class={nested ? "border-t border-line-subtle" : "flex flex-col rounded-lg border border-line-subtle"}
>
	{#each items as item (item.id)}
		<li class="border-b border-line-subtle last:border-b-0">
			<RoadmapItemRow {item} href={entryHref(item.id)} {window} class={nested ? "pl-8" : undefined} />
			{#if !nested && item.issues && item.issues.length > 0}
				<RoadmapItemList items={item.issues} {entryHref} nested label="Issues in {item.title}" />
			{/if}
		</li>
	{/each}
</ul>
