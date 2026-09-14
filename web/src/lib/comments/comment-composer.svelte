<script lang="ts">
	import { untrack } from "svelte";
	import AttachmentPicker from "$lib/attachments/attachment-picker.svelte";
	import DropOverlay from "$lib/attachments/drop-overlay.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import UploadList from "$lib/attachments/upload-list.svelte";
	import { dropZone } from "$lib/attachments/drop";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { attachmentNode } from "$lib/attachments/attachments";
	import { settled, type UploadTask } from "$lib/attachments/upload";
	import Editor from "$lib/editor/editor.svelte";
	import EmojiPicker from "$lib/editor/emoji-picker.svelte";
	import {
		documentEmpty,
		emptyDocument,
		type Document,
		type DocumentNode,
	} from "$lib/editor/document";

	let {
		workspaceId,
		workspace,
		working = false,
		body = emptyDocument,
		placeholder = "Leave a comment",
		submitLabel = "Comment",
		onsubmit,
		oncancel,
		uploads,
		onfiles,
		oncancelupload,
		onretryupload,
		ondismissupload,
	}: {
		workspaceId: string;
		workspace: string;
		working?: boolean;
		body?: Document;
		placeholder?: string;
		submitLabel?: string;
		onsubmit: (body: Document, attachmentIds: string[]) => Promise<boolean> | boolean;
		oncancel?: () => void;
		uploads?: UploadTask[];
		onfiles?: (files: File[]) => string[] | void;
		oncancelupload?: (id: string) => void;
		onretryupload?: (id: string) => void;
		ondismissupload?: (id: string) => void;
	} = $props();

	const id = $props.id();

	let draft = $state.raw<Document>(untrack(() => body));
	let dropping = $state(false);
	let sending = $state(false);
	let editor = $state.raw<{
		settle: (taskId: string, content: DocumentNode) => boolean;
		abandon: (taskId: string) => void;
		insert: (text: string) => void;
	} | null>(null);

	const empty = $derived(documentEmpty(draft));
	const inFlight = $derived((uploads ?? []).some((task) => !settled(task)));
	const attached = $derived(
		(uploads ?? []).filter((task) => task.state === "done" && task.attachment)
	);

	const placed = new Set<string>();

	$effect(() => {
		for (const task of uploads ?? []) {
			if (placed.has(task.id)) continue;

			if (task.state === "failed" || task.state === "cancelled") {
				placed.add(task.id);
				editor?.abandon(task.id);

				continue;
			}

			if (task.state !== "done" || !task.attachment) continue;

			placed.add(task.id);
			editor?.settle(task.id, attachmentNode(task.attachment));
		}
	});

	function take(files: File[]) {
		if (files.length === 0 || !onfiles) return;

		onfiles(files);
	}

	const zone = dropZone({
		take,
		over: (dragging) => (dropping = dragging),
		accepts: () => Boolean(onfiles) && !working && !sending,
	});

	async function send(): Promise<boolean> {
		if (empty || working || sending || inFlight) return false;

		sending = true;

		try {
			const attachmentIds = attached.map((task) => task.attachment?.id ?? "").filter(Boolean);
			const filed = await onsubmit(draft, attachmentIds);

			if (!filed) return false;

			if (!oncancel) draft = emptyDocument;

			return true;
		} finally {
			sending = false;
		}
	}
</script>

<div class="relative flex flex-col gap-2 rounded-lg" role="group" {...zone}>
	{#if dropping}
		<DropOverlay inset="-inset-1" />
	{/if}

	<Label for="comment-{id}" class="sr-only">{placeholder}</Label>
	<div class="rounded-md border border-line-default px-2.5 pb-1">
		<Editor
			bind:this={editor}
			bind:document={draft}
			id="comment-{id}"
			{workspaceId}
			{workspace}
			{placeholder}
			label={placeholder}
			disabled={working || sending}
			minHeight="min-h-17.5"
			emoji
			onfiles={onfiles ? (files) => onfiles(files) : undefined}
			onmetaenter={() => {
				void send();

				return true;
			}}
		/>
	</div>

	{#if uploads && uploads.length > 0}
		<UploadList
			{uploads}
			oncancel={(taskId) => oncancelupload?.(taskId)}
			onretry={(taskId) => onretryupload?.(taskId)}
			ondismiss={(taskId) => {
				editor?.abandon(taskId);
				ondismissupload?.(taskId);
			}}
		/>
	{/if}

	<div class="flex flex-wrap items-center gap-2">
		{#if onfiles}
			<AttachmentPicker disabled={working || sending} onfiles={take} iconOnly />
		{/if}

		<EmojiPicker disabled={working || sending} onpick={(glyph) => editor?.insert(glyph)} />

		<div class="flex-1"></div>

		{#if oncancel}
			<Button variant="ghost" size="sm" disabled={working || sending} onclick={oncancel}>
				Cancel
			</Button>
		{/if}

		<span class="hidden items-center gap-1.5 text-xs text-muted-foreground sm:inline-flex">
			<Kbd keys="⌘ ↵" /> {submitLabel.toLowerCase()}
		</span>

		<Button size="sm" disabled={working || sending || empty || inFlight} onclick={() => void send()}>
			{working || sending ? "Working" : inFlight ? "Uploading" : submitLabel}
		</Button>
	</div>
</div>
