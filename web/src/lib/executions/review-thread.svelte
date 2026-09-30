<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Markdown from "$lib/issues/markdown.svelte";
	import { onDateAndTime } from "$lib/time";
	import ReviewComposer from "./review-composer.svelte";
	import type { ReviewComment, ReviewThread } from "./review";

	let {
		thread,
		timezone,
		open,
		working,
		showHunk = false,
		onreply,
		onedit,
		ondelete,
		onresolve,
	}: {
		thread: ReviewThread;
		timezone: string;
		open: boolean;
		working: boolean;
		showHunk?: boolean;
		onreply: (parentId: string, body: string, publish: boolean) => Promise<boolean>;
		onedit: (commentId: string, body: string) => Promise<boolean>;
		ondelete: (commentId: string) => void;
		onresolve: (commentId: string, resolved: boolean) => void;
	} = $props();

	let replying = $state(false);
	let editing = $state<string | null>(null);
	let expanded = $state(false);

	const resolved = $derived(Boolean(thread.root.resolvedAt));
	const folded = $derived(resolved && !expanded);
	const comments = $derived([thread.root, ...thread.replies]);
</script>

{#snippet comment(held: ReviewComment)}
	<article class="flex min-w-0 flex-col gap-1.5 px-3 py-2.5">
		<header class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
			<span class="flex items-center gap-1 text-xs font-medium text-ink-900">
				{#if held.authorKind === "agent"}
					<Bot class="size-3.5 text-muted-foreground" aria-label="An agent wrote this" />
				{/if}
				{held.authorName || "Somebody"}
			</span>
			<time class="font-mono text-2xs text-muted-foreground" datetime={held.createdAt}>
				{onDateAndTime(held.createdAt, timezone)}
			</time>
			{#if held.editedAt}
				<span class="text-2xs text-muted-foreground">edited</span>
			{/if}
			{#if held.pending}
				<Eyebrow tone="attention">Draft</Eyebrow>
			{/if}
		</header>

		{#if editing === held.id}
			<ReviewComposer
				label="Edit your comment"
				initial={held.body}
				placeholder="Say what should change."
				primary={{ label: "Save", publish: false }}
				{working}
				onsubmit={async (body) => {
					const saved = await onedit(held.id, body);

					if (saved) editing = null;

					return saved;
				}}
				oncancel={() => (editing = null)}
			/>
		{:else}
			<Markdown source={held.body} class="text-sm" />

			{#if open && held.mine}
				<div class="flex flex-wrap gap-1">
					<Button variant="ghost" size="xs" disabled={working} onclick={() => (editing = held.id)}>
						Edit
					</Button>
					<Button variant="ghost" size="xs" disabled={working} onclick={() => ondelete(held.id)}>
						Delete
					</Button>
				</div>
			{/if}
		{/if}
	</article>
{/snippet}

<div
	class="flex min-w-0 flex-col rounded-sm border border-line-default bg-card font-sans whitespace-normal"
	aria-label={`Comment thread on line ${thread.root.line}`}
	role="group"
>
	{#if thread.root.outdated || resolved}
		<div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 border-b border-line-subtle px-3 py-1.5">
			{#if thread.root.outdated}
				<Eyebrow>Outdated</Eyebrow>
				<span class="text-2xs text-muted-foreground">
					Left on line {thread.root.line} of {thread.root.path}, which the run has since changed
				</span>
			{/if}
			{#if resolved}
				<Eyebrow tone="success">Resolved</Eyebrow>
				<span class="text-2xs text-muted-foreground">
					by {thread.root.resolvedByName || "somebody"}
				</span>
				<Button variant="ghost" size="xs" class="ml-auto" onclick={() => (expanded = !expanded)}>
					{expanded ? "Hide" : "Show"} the thread
				</Button>
			{/if}
		</div>
	{/if}

	{#if showHunk && thread.root.hunk}
		<pre
			class="overflow-x-auto border-b border-line-subtle bg-paper-1 px-3 py-2 font-mono text-2xs leading-relaxed text-muted-foreground">{thread.root.hunk}</pre>
	{/if}

	{#if !folded}
		<div class="flex min-w-0 flex-col divide-y divide-line-subtle">
			{#each comments as held (held.id)}
				{@render comment(held)}
			{/each}
		</div>

		{#if open}
			<div class="border-t border-line-subtle px-3 py-2">
				{#if replying}
					<ReviewComposer
						label="Reply to this thread"
						placeholder="Reply…"
						primary={{ label: "Add to review", publish: false }}
						secondary={{ label: "Reply now", publish: true }}
						{working}
						onsubmit={async (body, publish) => {
							const sent = await onreply(thread.root.id, body, publish);

							if (sent) replying = false;

							return sent;
						}}
						oncancel={() => (replying = false)}
					/>
				{:else}
					<div class="flex flex-wrap gap-1">
						<Button variant="ghost" size="xs" disabled={working} onclick={() => (replying = true)}>
							Reply
						</Button>
						{#if !thread.root.pending}
							<Button
								variant="ghost"
								size="xs"
								disabled={working}
								onclick={() => onresolve(thread.root.id, !resolved)}
							>
								{resolved ? "Open it again" : "Resolve"}
							</Button>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>
