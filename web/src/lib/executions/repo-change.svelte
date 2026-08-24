<script lang="ts">
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import { Button } from "$lib/components/ui/button/index.js";
	import CodeLinkPanel from "$lib/source-control/code-link-panel.svelte";
	import type { CodeLink } from "$lib/source-control/source-control";
	import DiffView from "./diff-view.svelte";
	import {
		diffReach,
		diffStatLine,
		noPullRequestLine,
		pullRequestReach,
		type DiffView as Diff,
		type ExecutionRepositoryChange,
	} from "./executions";

	let {
		change,
		links,
		diff,
		download,
		opened = false,
		ondiff,
	}: {
		change: ExecutionRepositoryChange;
		links: CodeLink[];
		diff: Diff;
		download: string;
		opened?: boolean;
		ondiff?: (artifactId: string) => void;
	} = $props();

	const reach = $derived(pullRequestReach(change, links));
	const patch = $derived(diffReach(change));
	const commits = $derived(change.commits === 1 ? "1 commit" : `${change.commits} commits`);
</script>

<li class="flex min-w-0 flex-col gap-1.5 border-b border-line-subtle py-2.5 last:border-b-0">
	<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
		<span class="font-mono text-xs text-ink-900">{change.repository}</span>
		<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
			{commits} · {diffStatLine(change)}
		</span>
	</div>

	{#if change.branch}
		<p class="flex min-w-0 items-center gap-1.5">
			<GitBranch aria-hidden="true" class="size-3 shrink-0 text-muted-foreground" />
			<span class="min-w-0 font-mono text-2xs break-all text-muted-foreground">
				{change.branch}
			</span>
		</p>
	{/if}

	{#if reach.kind === "linked"}
		<CodeLinkPanel links={[reach.link]} />
	{:else if reach.kind === "address"}
		<a
			href={reach.url}
			target="_blank"
			rel="noreferrer noopener"
			class="inline-flex min-w-0 items-center gap-1 text-xs break-all text-ink-900 underline underline-offset-2 hover:text-foreground"
		>
			{reach.url}
			<ExternalLink aria-hidden="true" class="size-3 shrink-0" />
		</a>
	{:else}
		<p class="text-2xs text-muted-foreground">{noPullRequestLine(change)}</p>
	{/if}

	{#if patch.kind === "available" && ondiff}
		<div class="flex flex-wrap items-center gap-2">
			<Button
				variant="ghost"
				size="sm"
				aria-expanded={opened}
				onclick={() => ondiff(patch.artifactId)}
			>
				{opened ? "Hide the diff" : "Show the diff"}
			</Button>
		</div>

		{#if opened}
			<DiffView view={diff} {download} />
		{/if}
	{:else if patch.kind === "available"}
		<a
			href={download}
			class="text-xs text-ink-900 underline underline-offset-2 hover:text-foreground"
		>
			Download the diff
		</a>
	{:else}
		<DiffView view={{ kind: "absent" }} {download} />
	{/if}
</li>
