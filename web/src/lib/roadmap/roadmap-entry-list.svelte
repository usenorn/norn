<script lang="ts">
	import ListPlus from "@lucide/svelte/icons/list-plus";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import RowGroup from "$lib/components/norn/row-group.svelte";
	import RoadmapEntryRow from "./roadmap-entry-row.svelte";
	import { entryLayout, type RoadmapActivity, type RoadmapEntry, type RoadmapVisibility } from "./admin";

	let {
		entries,
		visibility,
		activity,
		onedit,
		onrefresh,
		onremove,
	}: {
		entries: RoadmapEntry[];
		visibility: RoadmapVisibility;
		activity: RoadmapActivity;
		onedit: (entry: RoadmapEntry) => void;
		onrefresh: (entry: RoadmapEntry) => void;
		onremove: (entry: RoadmapEntry) => void;
	} = $props();

	const layout = $derived(entryLayout(entries));
</script>

{#snippet row(entry: RoadmapEntry)}
	<RoadmapEntryRow {entry} {visibility} {activity} {onedit} {onrefresh} {onremove} />
{/snippet}

{#if entries.length === 0}
	<Empty.Root>
		<Empty.Media variant="icon"><ListPlus aria-hidden="true" /></Empty.Media>
		<Empty.Header>
			<Empty.Title>Nothing selected yet</Empty.Title>
			<Empty.Description>
				Add a project or an issue, then write the public copy readers will see instead of your internal notes.
			</Empty.Description>
		</Empty.Header>
	</Empty.Root>
{:else}
	<div class="flex flex-col gap-4">
		{#if layout.projects.length > 0}
			<div class="flex flex-col gap-2">
				<h3><Eyebrow>Projects</Eyebrow></h3>
				<RowGroup as="ul">
					{#each layout.projects as group (group.project.id)}
						<li>
							{@render row(group.project)}
							{#if group.issues.length > 0}
								<ul
									class="ml-6 flex flex-col divide-y divide-line-subtle border-l border-line-subtle"
									aria-label="Issues listed under {group.project.publicTitle}"
								>
									{#each group.issues as issue (issue.id)}
										<li>{@render row(issue)}</li>
									{/each}
								</ul>
							{/if}
						</li>
					{/each}
				</RowGroup>
			</div>
		{/if}

		{#if layout.issues.length > 0}
			<div class="flex flex-col gap-2">
				<h3><Eyebrow>Issues on their own</Eyebrow></h3>
				<RowGroup as="ul">
					{#each layout.issues as issue (issue.id)}
						<li>{@render row(issue)}</li>
					{/each}
				</RowGroup>
			</div>
		{/if}
	</div>
{/if}
