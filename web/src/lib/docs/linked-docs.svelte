<script lang="ts">
	import FileText from "@lucide/svelte/icons/file-text";
	import type { Project } from "#lib/projects/projects.js";
	import LinkRow from "./link-row.svelte";
	import { docPath, docVisibilityLabels, projectName, type DocBacklinks } from "./docs";

	let {
		docs,
		workspace,
		projects,
	}: {
		docs: DocBacklinks;
		workspace: string;
		projects: Project[];
	} = $props();
</script>

{#if docs.kind === "unavailable"}
	<p role="status" class="text-sm leading-normal text-muted-foreground text-pretty">
		We could not load the docs linked here. Nothing changed — reload to try again.
	</p>
{:else}
	<ul class="flex flex-col">
		{#each docs.docs as doc (doc.id)}
			<LinkRow href={docPath(workspace, doc.id)}>
				<FileText class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
				<span class="min-w-0 flex-1 truncate text-md text-ink-900">{doc.title}</span>
				<span class="shrink-0 caption text-muted-foreground">
					{projectName(doc.audience, projects) ?? docVisibilityLabels[doc.audience.visibility]}
				</span>
			</LinkRow>
		{/each}
	</ul>
{/if}
