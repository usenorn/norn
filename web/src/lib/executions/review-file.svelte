<script lang="ts">
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import MessageSquarePlus from "@lucide/svelte/icons/message-square-plus";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import ReviewComposer from "./review-composer.svelte";
	import ReviewThread from "./review-thread.svelte";
	import {
		anchorOf,
		fileStatusLabel,
		hunkExcerpt,
		sameAnchor,
		segmentsOf,
		splitRows,
		type DiffAnchor,
		type DiffFile,
		type DiffSource,
		type DiffHunk,
		type DiffLine,
	} from "./diff";
	import { fileDiff } from "./diff.remote";
	import { fileBodyOf, outdatedThreads, tooLargeLine, type ReviewLayout, type ReviewThread as Thread } from "./review";

	let {
		id,
		file,
		origin,
		threads,
		layout,
		viewed,
		open,
		working,
		timezone,
		onviewed,
		oncomment,
		onreply,
		onedit,
		ondelete,
		onresolve,
	}: {
		id: string;
		file: DiffFile;
		origin: DiffSource | undefined;
		threads: Thread[];
		layout: ReviewLayout;
		viewed: boolean;
		open: boolean;
		working: boolean;
		timezone: string;
		onviewed: (viewed: boolean) => void;
		oncomment: (anchor: DiffAnchor, hunk: string, body: string, publish: boolean) => Promise<boolean>;
		onreply: (parentId: string, body: string, publish: boolean) => Promise<boolean>;
		onedit: (commentId: string, body: string) => Promise<boolean>;
		ondelete: (commentId: string) => void;
		onresolve: (commentId: string, resolved: boolean) => void;
	} = $props();

	let collapsed = $state<boolean | null>(null);
	let composing = $state<DiffAnchor | null>(null);
	let asked = $state(false);

	const load = $derived(asked && origin && file.deferred ? fileDiff({ ...origin, path: file.path }) : undefined);
	const body = $derived(
		fileBodyOf(file, load ? { current: load.current, loading: load.loading, error: load.error } : undefined)
	);
	const shown = $derived(body.kind === "inline" || body.kind === "ready" ? body.file : undefined);
	const download = $derived(
		origin
			? `/v1/workspaces/${origin.workspaceId}/executions/${origin.executionId}/artifacts/${origin.artifactId}/content`
			: undefined
	);

	const hidden = $derived(collapsed ?? viewed);
	const current = $derived(threads.filter((thread) => !thread.root.outdated));
	const outdated = $derived(outdatedThreads(threads));
	const status = $derived(fileStatusLabel(file));
	const rows = $derived(layout === "split" && shown ? splitRows(shown.hunks) : []);

	const gutter = {
		add: "bg-success/15 text-ink-600",
		remove: "bg-destructive/15 text-ink-600",
		context: "text-muted-foreground",
	};
	const code = {
		add: "bg-success/10 text-ink-900",
		remove: "bg-destructive/10 text-ink-900",
		context: "text-ink-900",
	};
	const marks = { add: "+", remove: "−", context: " " };

	function threadsOn(line: DiffLine | undefined): Thread[] {
		if (!line) return [];

		return current.filter(
			(thread) =>
				(thread.root.side === "new" && line.newLine === thread.root.line && line.kind !== "remove") ||
				(thread.root.side === "old" && line.oldLine === thread.root.line && line.kind !== "add")
		);
	}

	function composingOn(line: DiffLine | undefined): boolean {
		return Boolean(line && composing && sameAnchor(composing, anchorOf(line)));
	}

	function pairThreads(left: DiffLine | undefined, right: DiffLine | undefined): Thread[] {
		const found = new Map<string, Thread>();

		for (const thread of [...threadsOn(left), ...threadsOn(right)]) found.set(thread.root.id, thread);

		return [...found.values()];
	}

	function hunkOf(line: DiffLine): DiffHunk {
		const hunks = shown?.hunks ?? [];

		return hunks.find((hunk) => hunk.lines.includes(line)) ?? hunks[0];
	}

	function nearView(node: HTMLElement) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries.some((entry) => entry.isIntersecting)) asked = true;
			},
			{ rootMargin: "600px 0px" }
		);

		observer.observe(node);

		return () => observer.disconnect();
	}

	function numberOf(line: DiffLine): number {
		return anchorOf(line).line;
	}
</script>

{#snippet source(line: DiffLine | undefined)}{#if line?.spans}{#each segmentsOf(line) as segment, index (index)}<span style:color={segment.tone}>{segment.text}</span>{/each}{:else}{line?.text ?? ""}{/if}{/snippet}

{#snippet commentButton(line: DiffLine)}
	{#if open}
		<button
			type="button"
			class="absolute top-0 left-0.5 inline-flex size-4 items-center justify-center rounded-xs bg-primary text-primary-foreground opacity-0 group-hover/line:opacity-100 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-ring [@media(hover:none)]:opacity-60"
			aria-label={`Comment on ${anchorOf(line).side === "old" ? "old" : "new"} line ${numberOf(line)}`}
			disabled={working}
			onclick={() => (composing = anchorOf(line))}
		>
			<MessageSquarePlus class="size-3" aria-hidden="true" />
		</button>
	{/if}
{/snippet}

{#snippet composer(line: DiffLine)}
	<ReviewComposer
		label={`Comment on line ${numberOf(line)} of ${file.path}`}
		placeholder="Say what should change, and why."
		primary={{ label: "Add to review", publish: false }}
		secondary={{ label: "Comment now", publish: true }}
		{working}
		onsubmit={async (body, publish) => {
			const sent = await oncomment(anchorOf(line), hunkExcerpt(hunkOf(line), line), body, publish);

			if (sent) composing = null;

			return sent;
		}}
		oncancel={() => (composing = null)}
	/>
{/snippet}

{#snippet discussion(found: Thread[], line: DiffLine | undefined, span: number)}
	{#if found.length > 0 || composingOn(line)}
		<tr>
			<td colspan={span} class="border-y border-line-subtle bg-paper-1 px-3 py-2.5">
				<div class="flex max-w-3xl min-w-0 flex-col gap-2.5">
					{#each found as thread (thread.root.id)}
						<ReviewThread
							{thread}
							{timezone}
							{open}
							{working}
							{onreply}
							{onedit}
							{ondelete}
							{onresolve}
						/>
					{/each}
					{#if line && composingOn(line)}
						{@render composer(line)}
					{/if}
				</div>
			</td>
		</tr>
	{/if}
{/snippet}

<section
	{id}
	class="flex min-w-0 scroll-mt-(--review-bar) flex-col rounded-sm border border-line-default bg-card"
	aria-label={file.path}
>
	<header
		class="sticky top-(--review-bar) z-10 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 rounded-t-sm border-b border-line-default bg-card px-2 py-1.5"
	>
		<button
			type="button"
			class="inline-flex min-w-0 flex-1 items-center gap-1.5 rounded-xs text-left focus-visible:outline-2 focus-visible:outline-ring"
			aria-expanded={!hidden}
			aria-controls={`${id}-diff`}
			onclick={() => (collapsed = !hidden)}
		>
			{#if hidden}
				<ChevronRight class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
			{:else}
				<ChevronDown class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
			{/if}
			<span class="min-w-0 font-mono text-xs break-all text-ink-900">{file.path}</span>
		</button>

		{#if status}
			<Eyebrow>{status}</Eyebrow>
		{/if}

		<span class="font-mono text-2xs whitespace-nowrap">
			<span class="text-success">+{file.additions}</span>
			<span class="text-destructive">−{file.deletions}</span>
		</span>

		{#if threads.length > 0}
			<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
				{threads.length === 1 ? "1 thread" : `${threads.length} threads`}
			</span>
		{/if}

		<div class="flex items-center gap-1.5">
			<Checkbox
				id={`${id}-viewed`}
				checked={viewed}
				onCheckedChange={(checked) => onviewed(checked === true)}
			/>
			<Label for={`${id}-viewed`} class="text-2xs text-muted-foreground">Viewed</Label>
		</div>
	</header>

	{#if !hidden}
		<div id={`${id}-diff`} class="overflow-hidden rounded-b-sm">
			{#if outdated.length > 0}
				<div class="flex min-w-0 flex-col gap-2.5 border-b border-line-subtle bg-paper-1 px-3 py-2.5">
					{#each outdated as thread (thread.root.id)}
						<ReviewThread
							{thread}
							{timezone}
							{open}
							{working}
							showHunk
							{onreply}
							{onedit}
							{ondelete}
							{onresolve}
						/>
					{/each}
				</div>
			{/if}

			{#if file.binary}
				<p class="px-3 py-3 text-xs text-muted-foreground">
					A binary file, so there is nothing to read line by line.
				</p>
			{:else if !shown}
				<div class="flex flex-wrap items-center gap-2 px-3 py-3 text-xs text-muted-foreground" aria-live="polite" {@attach nearView}>
					{#if body.kind === "waiting"}
						<span>These changes load when you reach them.</span>
						<Button variant="outline" size="xs" onclick={() => (asked = true)}>Show the changes</Button>
					{:else if body.kind === "loading"}
						<span>Loading the changes…</span>
					{:else if body.kind === "too_large"}
						<span>{tooLargeLine(body.lines)}</span>
						{#if download}
							<a href={download} class="underline underline-offset-2 hover:text-foreground">Download the full diff</a>
						{/if}
					{:else}
						<span>The changes could not be loaded.</span>
						<Button variant="outline" size="xs" onclick={() => load?.refresh()}>Try again</Button>
					{/if}
				</div>
			{:else if shown.hunks.length === 0}
				<p class="px-3 py-3 text-xs text-muted-foreground">
					{file.status === "renamed" ? "Moved without changing a line." : "No lines changed."}
				</p>
			{:else if layout === "unified"}
				<div class="overflow-x-auto overflow-y-hidden">
					<table class="w-full border-collapse font-mono text-2xs leading-relaxed">
						<tbody>
							{#each shown.hunks as hunk, hunkIndex (hunkIndex)}
								<tr>
									<td colspan="3" class="bg-paper-1 px-3 py-1 text-muted-foreground whitespace-pre">
										{hunk.header}
									</td>
								</tr>
								{#each hunk.lines as line, lineIndex (lineIndex)}
									<tr class="group/line">
										<td class="relative w-10 min-w-10 px-2 text-right select-none {gutter[line.kind]}">
											{@render commentButton(line)}
											{line.oldLine ?? ""}
										</td>
										<td class="w-10 min-w-10 px-2 text-right select-none {gutter[line.kind]}">
											{line.newLine ?? ""}
										</td>
										<td class="px-2 whitespace-pre {code[line.kind]}"
											><span class="select-none text-muted-foreground">{marks[line.kind]}</span>{@render source(line)}</td
										>
									</tr>
									{@render discussion(threadsOn(line), line, 3)}
								{/each}
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="overflow-x-auto overflow-y-hidden">
					<table class="w-full table-fixed border-collapse font-mono text-2xs leading-relaxed">
						<colgroup>
							<col class="w-10" />
							<col />
							<col class="w-10" />
							<col />
						</colgroup>
						<tbody>
							{#each rows as row, rowIndex (rowIndex)}
								{#if row.kind === "header"}
									<tr>
										<td colspan="4" class="bg-paper-1 px-3 py-1 text-muted-foreground whitespace-pre">
											{row.header}
										</td>
									</tr>
								{:else}
									<tr class="group/line">
										<td
											class="relative px-2 text-right select-none {row.left ? gutter[row.left.kind] : 'bg-paper-1'}"
										>
											{#if row.left && row.left.kind === "remove"}
												{@render commentButton(row.left)}
											{/if}
											{row.left?.oldLine ?? ""}
										</td>
										<td
											class="px-2 break-all whitespace-pre-wrap {row.left
												? code[row.left.kind]
												: 'bg-paper-1'}">{@render source(row.left)}</td
										>
										<td
											class="relative px-2 text-right select-none {row.right ? gutter[row.right.kind] : 'bg-paper-1'}"
										>
											{#if row.right}
												{@render commentButton(row.right)}
											{/if}
											{row.right?.newLine ?? ""}
										</td>
										<td
											class="px-2 break-all whitespace-pre-wrap {row.right
												? code[row.right.kind]
												: 'bg-paper-1'}">{@render source(row.right)}</td
										>
									</tr>
									{@render discussion(
										pairThreads(row.left, row.right),
										composingOn(row.left) ? row.left : row.right,
										4
									)}
								{/if}
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</section>
