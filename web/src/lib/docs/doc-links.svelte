<script lang="ts">
	import Folder from "@lucide/svelte/icons/folder";
	import Link2 from "@lucide/svelte/icons/link-2";
	import { Button } from "#lib/components/ui/button/index.js";
	import StatusIcon from "#lib/components/norn/status-icon.svelte";
	import PropertyPicker, { type PickerOption } from "#lib/issues/property-picker.svelte";
	import type { IssueCandidate } from "#lib/issues/issues.js";
	import type { Project } from "#lib/projects/projects.js";
	import { workspacePath } from "#lib/workspace/navigation.js";
	import LinkRow from "./link-row.svelte";
	import { unlinkedIssues, unlinkedProjects, type DocLinks } from "./docs";

	let {
		links,
		candidates,
		projects,
		workspace,
		editable,
		working = false,
		onlink,
		onunlink,
	}: {
		links: DocLinks;
		candidates: IssueCandidate[];
		projects: Project[];
		workspace: string;
		editable: boolean;
		working?: boolean;
		onlink: (target: { issueId: string } | { projectId: string }) => void;
		onunlink: (linkId: string) => void;
	} = $props();

	let issueOpen = $state(false);
	let projectOpen = $state(false);

	const held = $derived(links.kind === "ready" ? links.links : []);

	const issueOptions = $derived(
		unlinkedIssues(held, candidates).map(
			(candidate): PickerOption => ({
				value: candidate.id,
				label: `${candidate.reference} ${candidate.title}`,
				hint: candidate.state.name,
			})
		)
	);

	const projectOptions = $derived(
		unlinkedProjects(held, projects).map((project): PickerOption => ({ value: project.id, label: project.name }))
	);
</script>

<div class="flex flex-col gap-1.5">
	{#if links.kind === "unavailable"}
		<p role="status" class="text-sm leading-normal text-muted-foreground text-pretty">
			We could not load what this doc links to. Nothing changed — reload to try again.
		</p>
	{:else if held.length > 0}
		<ul class="flex flex-col">
			{#each held as link (link.id)}
				{#if link.issue}
					<LinkRow
						href={workspacePath(workspace, `/issues/${link.issue.reference}`)}
						removeLabel="Unlink {link.issue.reference}"
						{working}
						onremove={editable ? () => onunlink(link.id) : undefined}
					>
						<StatusIcon category={link.issue.state.category} name={link.issue.state.name} />
						<span class="font-mono text-xs text-muted-foreground">{link.issue.reference}</span>
						<span class="min-w-0 flex-1 truncate text-md text-ink-900">{link.issue.title}</span>
					</LinkRow>
				{:else if link.project}
					<LinkRow
						href={workspacePath(workspace, `/projects/${link.project.slug}`)}
						removeLabel="Unlink {link.project.name}"
						{working}
						onremove={editable ? () => onunlink(link.id) : undefined}
					>
						<Folder class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
						<span class="min-w-0 flex-1 truncate text-md text-ink-900">{link.project.name}</span>
						{#if link.project.archived}
							<span class="shrink-0 caption text-muted-foreground">Archived</span>
						{/if}
					</LinkRow>
				{/if}
			{/each}
		</ul>
	{:else if !editable}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This doc does not link to any issue or project.
		</p>
	{/if}

	{#if editable && links.kind === "ready"}
		<div class="flex flex-wrap items-center gap-1.5 pt-0.5">
			<PropertyPicker
				options={issueOptions}
				placeholder="Link an issue…"
				empty="No issues left to link"
				bind:open={issueOpen}
				onpick={(issueId) => onlink({ issueId })}
			>
				{#snippet trigger(props)}
					<Button {...props} variant="ghost" size="sm" disabled={working}>
						<Link2 aria-hidden="true" />
						Link an issue
					</Button>
				{/snippet}
			</PropertyPicker>
			<PropertyPicker
				options={projectOptions}
				placeholder="Link a project…"
				empty="No projects left to link"
				bind:open={projectOpen}
				onpick={(projectId) => onlink({ projectId })}
			>
				{#snippet trigger(props)}
					<Button {...props} variant="ghost" size="sm" disabled={working}>
						<Folder aria-hidden="true" />
						Link a project
					</Button>
				{/snippet}
			</PropertyPicker>
		</div>
	{/if}
</div>
