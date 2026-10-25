<script lang="ts">
	import GitMerge from "@lucide/svelte/icons/git-merge";
	import Replace from "@lucide/svelte/icons/replace";
	import { Button } from "#lib/components/ui/button/index.js";
	import Pill from "#lib/components/norn/pill.svelte";
	import type { Project } from "#lib/projects/projects.js";
	import {
		memoryKindLabels,
		memoryPath,
		memoryRelationLabels,
		memoryRelationLines,
		memoryRelationTones,
		memoryStatusLabels,
		memoryStatusTones,
		scopeLine,
		type MemoryResolution,
		type RelatedMemory,
	} from "./memories";

	let {
		related,
		workspace,
		projects,
		offered,
		working,
		onchoose,
	}: {
		related: RelatedMemory;
		workspace: string;
		projects: Project[];
		offered: MemoryResolution[];
		working: boolean;
		onchoose: (resolution: MemoryResolution) => void;
	} = $props();

	const memory = $derived(related.memory);
</script>

<div class="flex flex-col gap-2 px-3 py-3">
	<div class="flex flex-wrap items-center gap-2">
		<Pill tone={memoryRelationTones[related.relation]}>{memoryRelationLabels[related.relation]}</Pill>
		<Pill tone={memoryStatusTones[memory.status]}>{memoryStatusLabels[memory.status]}</Pill>
	</div>
	<a
		href={memoryPath(workspace, memory.id)}
		class="text-sm font-medium text-pretty break-words text-ink-900 underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
	>
		{memory.statement}
	</a>
	<p class="text-xs leading-normal text-muted-foreground">
		{memoryKindLabels[memory.kind]} · {scopeLine(memory, projects)}{memory.conditions
			? ` · Holds when ${memory.conditions}`
			: ""}
	</p>
	<p class="text-xs leading-normal text-muted-foreground">{memoryRelationLines[related.relation]}</p>
	{#if offered.length > 0}
		<div class="flex flex-wrap gap-1.5">
			{#if offered.includes("update")}
				<Button variant="secondary" size="sm" disabled={working} onclick={() => onchoose("update")}>
					<GitMerge aria-hidden="true" />
					Update it with this
				</Button>
			{/if}
			{#if offered.includes("supersede")}
				<Button variant="secondary" size="sm" disabled={working} onclick={() => onchoose("supersede")}>
					<Replace aria-hidden="true" />
					Supersede it
				</Button>
			{/if}
		</div>
	{/if}
</div>
