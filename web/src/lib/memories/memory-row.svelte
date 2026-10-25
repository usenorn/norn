<script lang="ts">
	import Pill from "#lib/components/norn/pill.svelte";
	import type { Project } from "#lib/projects/projects.js";
	import {
		memoryKindLabels,
		memoryPath,
		memoryStatusLabels,
		memoryStatusTones,
		scopeLine,
		type Memory,
	} from "./memories";

	let { memory, workspace, projects }: { memory: Memory; workspace: string; projects: Project[] } = $props();
</script>

<a
	href={memoryPath(workspace, memory.id)}
	class="flex min-w-0 flex-col gap-1 px-3 py-2.5 motion-row hover:bg-accent focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring sm:flex-row sm:items-center sm:gap-3"
>
	<span class="min-w-0 flex-1 text-sm text-pretty break-words text-ink-900">{memory.statement}</span>
	<span class="flex flex-none flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
		<span>{memoryKindLabels[memory.kind]}</span>
		<span aria-hidden="true">·</span>
		<span class="max-w-60 truncate">{scopeLine(memory, projects)}</span>
		<Pill tone={memoryStatusTones[memory.status]}>{memoryStatusLabels[memory.status]}</Pill>
	</span>
</a>
