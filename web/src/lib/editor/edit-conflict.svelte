<script lang="ts">
	import { Button } from "#lib/components/ui/button/index.js";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import Editor from "./editor.svelte";
	import type { Document } from "./document";

	let {
		mine,
		theirs,
		workspaceId,
		workspace,
		summary,
		mineLabel,
		theirsLabel,
		busy = false,
		onkeep,
		ontake,
	}: {
		mine: Document;
		theirs: Document;
		workspaceId: string;
		workspace: string;
		summary: string;
		mineLabel: string;
		theirsLabel: string;
		busy?: boolean;
		onkeep: () => void;
		ontake: () => void;
	} = $props();
</script>

<div class="flex flex-col gap-2 rounded-md border border-line-strong p-2.5" role="group" aria-label="Conflicting saves">
	<p class="text-sm text-ink-900">{summary}</p>
	<div class="grid gap-2.5 sm:grid-cols-2">
		<div class="flex min-w-0 flex-col gap-1">
			<Eyebrow class="text-ink-600">Yours</Eyebrow>
			<div class="max-h-40 min-w-0 overflow-y-auto rounded-sm border border-line-subtle p-2">
				<Editor document={mine} {workspaceId} {workspace} disabled minHeight="min-h-0" label={mineLabel} />
			</div>
		</div>
		<div class="flex min-w-0 flex-col gap-1">
			<Eyebrow class="text-ink-600">Theirs</Eyebrow>
			<div class="max-h-40 min-w-0 overflow-y-auto rounded-sm border border-line-subtle p-2">
				<Editor document={theirs} {workspaceId} {workspace} disabled minHeight="min-h-0" label={theirsLabel} />
			</div>
		</div>
	</div>
	<div class="flex flex-wrap items-center gap-2">
		<Button type="button" size="sm" disabled={busy} onclick={onkeep}>Keep mine</Button>
		<Button type="button" variant="secondary" size="sm" disabled={busy} onclick={ontake}>Take theirs</Button>
	</div>
</div>
