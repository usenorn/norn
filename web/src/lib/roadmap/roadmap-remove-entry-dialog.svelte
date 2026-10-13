<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import type { RoadmapEntry } from "./admin";

	let {
		entry,
		entries,
		removing,
		onconfirm,
		oncancel,
	}: {
		entry: RoadmapEntry | undefined;
		entries: RoadmapEntry[];
		removing: boolean;
		onconfirm: (entry: RoadmapEntry) => void;
		oncancel: () => void;
	} = $props();

	const nested = $derived(
		entry?.kind === "project"
			? entries.filter((candidate) => candidate.kind === "issue" && candidate.parentEntryId === entry.id).length
			: 0
	);
	const named = $derived(entry?.publicTitle || entry?.source?.label || "this entry");
</script>

<AlertDialog.Root
	open={entry !== undefined}
	onOpenChange={(next) => {
		if (!next && !removing) oncancel();
	}}
>
	<AlertDialog.Content size="sm">
		<AlertDialog.Header>
			<Eyebrow tone={entry?.published ? "danger" : "muted"}>{entry?.published ? "Leaves the public page now" : "Not public yet"}</Eyebrow>
			<AlertDialog.Title>Remove {named}</AlertDialog.Title>
			<AlertDialog.Description>
				{#if entry?.published}
					It disappears from the draft and from the public roadmap at once. You do not need to publish.
				{:else}
					It disappears from the draft. Nothing public changes.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if nested > 0}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				{nested === 1 ? "The issue" : `The ${nested} issues`} selected under it stay on the roadmap, listed under Other
				planned work.
			</p>
		{/if}

		<AlertDialog.Footer>
			{#if removing}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action
				variant="destructive"
				disabled={removing}
				onclick={() => {
					if (entry) onconfirm(entry);
				}}
			>
				{removing ? "Removing" : "Remove"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
