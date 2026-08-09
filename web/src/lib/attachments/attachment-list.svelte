<script lang="ts">
	import Download from "@lucide/svelte/icons/download";
	import ImageIcon from "@lucide/svelte/icons/image";
	import Paperclip from "@lucide/svelte/icons/paperclip";
	import X from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button/index.js";
	import { formatBytes, type AttachmentPanel } from "$lib/attachments/attachments";

	let {
		panel,
		working = false,
		onremove,
	}: { panel: AttachmentPanel; working?: boolean; onremove: (attachmentId: string) => void } =
		$props();
</script>

{#if panel.kind === "loading"}
	<div class="h-12 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
{:else if panel.kind === "unavailable"}
	<p class="text-sm text-muted-foreground">We could not read this issue's files.</p>
{:else if panel.kind === "empty"}
	<p class="text-sm text-muted-foreground">No files yet.</p>
{:else}
	<ul class="flex flex-col rounded-lg border border-line-default">
		{#each panel.attachments as attachment (attachment.id)}
			<li class="flex items-center gap-2 border-b border-line-subtle pr-1 last:border-b-0">
				<a
					href={attachment.contentPath}
					download={attachment.fileName}
					class="flex min-w-0 flex-1 items-center gap-2 py-2 pl-3 motion-control hover:bg-accent"
				>
					{#if attachment.inline}
						<ImageIcon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
					{:else}
						<Paperclip class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
					{/if}
					<span class="min-w-0 flex-1 truncate text-md tracking-snug text-ink-900">
						{attachment.fileName}
					</span>
					<span class="shrink-0 font-mono text-xs text-muted-foreground">
						{formatBytes(attachment.byteSize)}
					</span>
					<Download class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
				</a>
				<Button
					variant="ghost"
					size="sm"
					disabled={working}
					aria-label="Remove {attachment.fileName}"
					onclick={() => onremove(attachment.id)}
				>
					<X aria-hidden="true" />
				</Button>
			</li>
		{/each}
	</ul>
{/if}
