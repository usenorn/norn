<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import CircleX from "@lucide/svelte/icons/circle-x";
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
		class="flex scroll-mt-16 flex-col gap-2 rounded-sm target:rule-inset target:bg-accent {reply
			? 'border-l border-line-subtle pl-4'
			: ''}"
	>
		<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
			<span class="flex items-center gap-1 text-sm font-medium text-ink-900">
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

<section class="flex flex-col gap-4">
	<h2 class="text-sm font-medium text-ink-900">Comments</h2>

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
	{:else if thread.kind === "empty"}
		<p class="text-sm text-muted-foreground">Nobody has said anything yet.</p>
	{:else if ready}
		<ul class="flex flex-col gap-6">
			{#each ready.comments as comment (comment.id)}
				{@render entry(comment, false)}
			{/each}
		</ul>

		{#if ready.nextCursor}
			<div>
				<Button variant="secondary" size="sm" disabled={working} onclick={onmore}>
					{working ? "Loading" : "Load earlier comments"}
				</Button>
			</div>
		{/if}
	{/if}

	<CommentComposer
		{members}
		{teams}
		{working}
		{uploads}
		{onfiles}
		{oncancelupload}
		{onretryupload}
		{ondismissupload}
		placeholder="Leave a comment"
		submitLabel="Comment"
		onsubmit={(body, mentions, attachmentIds) => onpost(body, mentions, attachmentIds)}
	/>
</section>
