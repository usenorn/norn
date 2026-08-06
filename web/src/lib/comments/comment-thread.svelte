<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Dot from "@lucide/svelte/icons/dot";
	import Info from "@lucide/svelte/icons/info";
	import Reply from "@lucide/svelte/icons/reply";
	import SmilePlus from "@lucide/svelte/icons/smile-plus";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Markdown from "$lib/issues/markdown.svelte";
	import CommentComposer from "$lib/comments/comment-composer.svelte";
	import {
		authorLabel,
		commentFailureMessage,
		reacted,
		reactionGlyphs,
		reactionLabels,
		reactions,
		unreachableLine,
		type CommentFailure,
		type CommentMention,
		type CommentReaction,
		type CommentThread,
		type IssueComment,
		type MentionTarget,
	} from "$lib/comments/comments";
	import type { UploadTask } from "$lib/attachments/upload";
	import type { Member } from "$lib/issues/members";
	import type { Team } from "$lib/team/teams";

	let {
		thread,
		events = [],
		canEdit = true,
		lockedLine = "",
		members,
		teams,
		accountId,
		when,
		working = false,
		failure = null,
		unreachable = [],
		onpost,
		onedit,
		onremove,
		onreact,
		onmore,
		uploads,
		onfiles,
		oncancelupload,
		onretryupload,
		ondismissupload,
	}: {
		thread: CommentThread;
		events?: { id: string; at: string; line: string }[];
		canEdit?: boolean;
		lockedLine?: string;
		members: Member[];
		teams: Team[];
		accountId: string;
		when: (instant: string) => string;
		working?: boolean;
		failure?: CommentFailure | null;
		unreachable?: CommentMention[];
		onpost: (
			body: string,
			mentions: MentionTarget[],
			attachmentIds: string[],
			parentCommentId?: string
		) => void;
		onedit: (commentId: string, body: string) => void;
		onremove: (commentId: string) => void;
		onreact: (commentId: string, reaction: CommentReaction, on: boolean) => void;
		onmore: () => void;
		uploads?: UploadTask[];
		onfiles?: (files: File[]) => void;
		oncancelupload?: (id: string) => void;
		onretryupload?: (id: string) => void;
		ondismissupload?: (id: string) => void;
	} = $props();

	let replyingTo = $state("");
	let editing = $state("");

	const ready = $derived(thread.kind === "ready" ? thread : null);
	const missed = $derived(unreachableLine(unreachable));
	const stream = $derived(
		[
			...(ready?.comments ?? []).map((comment) => ({ at: comment.createdAt, comment, event: null })),
			...events.map((event) => ({ at: event.at, comment: null, event })),
		].sort((first, second) => first.at.localeCompare(second.at))
	);

	function editable(comment: IssueComment): boolean {
		return !comment.deleted && comment.authorAccountId === accountId;
	}

	function removable(comment: IssueComment): boolean {
		return !comment.deleted && comment.authorAccountId === accountId;
	}
</script>

{#snippet entry(comment: IssueComment, reply: boolean)}
	<li
		id="comment-{comment.id}"
		class="flex scroll-mt-16 flex-col gap-1.75 rounded-sm py-3.25 target:rule-inset target:bg-accent {reply
			? 'border-l border-line-subtle pl-4'
			: 'border-t border-line-subtle'}"
	>
		<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
			<span class="flex items-center gap-1 text-md font-medium tracking-snug text-ink-900">
				{#if comment.authorKind === "agent"}
					<Bot class="size-3.5 text-muted-foreground" aria-label="An agent wrote this" />
				{/if}
				{authorLabel(comment)}
			</span>
			<span class="font-mono text-xs text-muted-foreground">
				<time datetime={comment.createdAt}>{when(comment.createdAt)}</time>
			</span>
			{#if comment.edited}
				<span class="text-xs text-muted-foreground">
					edited{comment.editedAt ? ` ${when(comment.editedAt)}` : ""}
				</span>
			{/if}
		</div>

		{#if comment.deleted}
			<p class="text-sm text-muted-foreground italic">This comment was deleted.</p>
		{:else if editing === comment.id}
			<CommentComposer
				{members}
				{teams}
				{working}
				body={comment.body}
				placeholder="Edit your comment"
				submitLabel="Save"
				onsubmit={(body) => {
					editing = "";
					onedit(comment.id, body);
				}}
				oncancel={() => (editing = "")}
			/>
		{:else}
			<Markdown source={comment.body} />

			{#if comment.mentions.length > 0}
				<p class="text-xs text-muted-foreground">
					Mentioned {comment.mentions.map((mention) => mention.name).join(", ")}
				</p>
			{/if}
		{/if}

		<div class="flex flex-wrap items-center gap-1">
			{#each comment.reactions as tally (tally.reaction)}
				<Button
					variant={reacted(tally, accountId) ? "secondary" : "ghost"}
					size="sm"
					disabled={working || comment.deleted}
					aria-pressed={reacted(tally, accountId)}
					aria-label="{reactionLabels[tally.reaction]} ({tally.accountIds.length})"
					onclick={() => onreact(comment.id, tally.reaction, !reacted(tally, accountId))}
				>
					<span aria-hidden="true">{reactionGlyphs[tally.reaction]}</span>
					{tally.accountIds.length}
				</Button>
			{/each}

			{#if !comment.deleted}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="ghost" size="sm" disabled={working} aria-label="React">
								<SmilePlus aria-hidden="true" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="start">
						{#each reactions as reaction (reaction)}
							<DropdownMenu.Item onSelect={() => onreact(comment.id, reaction, true)}>
								<span aria-hidden="true">{reactionGlyphs[reaction]}</span>
								{reactionLabels[reaction]}
							</DropdownMenu.Item>
						{/each}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}

			{#if !reply}
				<Button
					variant="ghost"
					size="sm"
					disabled={working}
					onclick={() => (replyingTo = replyingTo === comment.id ? "" : comment.id)}
				>
					<Reply aria-hidden="true" />
					Reply
				</Button>
			{/if}

			{#if editable(comment)}
				<Button
					variant="ghost"
					size="sm"
					disabled={working}
					onclick={() => (editing = editing === comment.id ? "" : comment.id)}
				>
					Edit
				</Button>
			{/if}

			{#if removable(comment)}
				<Button variant="ghost" size="sm" disabled={working} onclick={() => onremove(comment.id)}>
					Delete
				</Button>
			{/if}
		</div>

		{#if comment.replies.length > 0}
			<ul class="flex flex-col gap-4">
				{#each comment.replies as child (child.id)}
					{@render entry(child, true)}
				{/each}
			</ul>
		{/if}

		{#if replyingTo === comment.id}
			<div class="border-l border-line-subtle pl-4">
				<CommentComposer
					{members}
					{teams}
					{working}
					placeholder="Reply to {authorLabel(comment)}"
					submitLabel="Reply"
					onsubmit={(body, mentions, attachmentIds) => {
						replyingTo = "";
						onpost(body, mentions, attachmentIds, comment.id);
					}}
					oncancel={() => (replyingTo = "")}
				/>
			</div>
		{/if}
	</li>
{/snippet}

<div class="flex flex-col gap-2">
	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{commentFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if missed}
		<Alert.Root>
			<Alert.Title>Some people were not notified</Alert.Title>
			<Alert.Description>{missed}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if thread.kind === "loading"}
		<div class="h-24 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if thread.kind === "unavailable"}
		<p class="text-sm text-muted-foreground">We could not read this conversation.</p>
	{:else}
		{#if ready?.nextCursor}
			<button
				type="button"
				disabled={working}
				onclick={onmore}
				class="flex h-7 w-full cursor-pointer items-center justify-center rounded-md border border-line-default font-mono text-xs text-muted-foreground transition-colors duration-70 ease-out hover:bg-accent hover:text-ink-600"
			>
				{working ? "Loading" : "Load earlier comments"}
			</button>
		{/if}

		{#if stream.length > 0}
			<ul class="flex flex-col">
				{#each stream as item (item.comment?.id ?? item.event?.id)}
					{#if item.comment}
						{@render entry(item.comment, false)}
					{:else if item.event}
						<li class="flex h-7 items-center gap-2.25">
							<span class="inline-flex w-4 flex-none justify-center">
								<Dot class="size-3 text-muted-foreground" aria-hidden="true" />
							</span>
							<span class="min-w-0 truncate text-sm text-ink-600">{item.event.line}</span>
							<span class="min-w-2 flex-1"></span>
							<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
								<time datetime={item.event.at}>{when(item.event.at)}</time>
							</span>
						</li>
					{/if}
				{/each}
			</ul>
		{/if}

		{#if thread.kind === "empty"}
			<div class="flex flex-col gap-1.5 border-t border-line-subtle pt-5.5 pb-4.5">
				<span class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">
					No comments yet
				</span>
				<span class="text-md text-muted-foreground">
					Ask a question or leave what you found. Everyone watching this issue is notified.
				</span>
			</div>
		{/if}
	{/if}

	{#if canEdit}
		<div class="mt-1.5 flex gap-2.75 border-t border-line-default pt-4">
			<div class="min-w-0 flex-1">
				<CommentComposer
					{members}
					{teams}
					{working}
					{uploads}
					{onfiles}
					{oncancelupload}
					{onretryupload}
					{ondismissupload}
					placeholder="Write a comment"
					submitLabel="Comment"
					onsubmit={(body, mentions, attachmentIds) => onpost(body, mentions, attachmentIds)}
				/>
			</div>
		</div>
	{:else if lockedLine}
		<div
			class="mt-2 flex min-h-11 items-center gap-2.25 rounded-md border border-line-default px-2.75"
		>
			<Info class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
			<span class="text-md text-muted-foreground">{lockedLine}</span>
		</div>
	{/if}
</div>
