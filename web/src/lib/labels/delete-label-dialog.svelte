<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { issueCount, mergeTargets, type Label } from "./labels";

	let {
		open = $bindable(false),
		label,
		labels,
		usage,
		removing,
		onconfirm,
		onmergeinstead,
	}: {
		open?: boolean;
		label: Label | null;
		labels: Label[];
		usage: number | null;
		removing: boolean;
		onconfirm: () => void;
		onmergeinstead: () => void;
	} = $props();

	const canMerge = $derived(label ? mergeTargets(label, labels).length > 0 : false);
	const inUse = $derived(usage !== null && usage > 0);
</script>

<AlertDialog.Root
	{open}
	onOpenChange={(next) => {
		if (!next && !removing) open = false;
	}}
>
	<AlertDialog.Content size="sm">
		<AlertDialog.Header>
			<Eyebrow tone={inUse ? "danger" : "muted"}>{inUse ? "In use" : "Not in use"}</Eyebrow>
			<AlertDialog.Title class="flex flex-wrap items-center gap-2">
				<span>Delete</span>
				{#if label}
					<Tag name={label.name} color={label.color} />
				{/if}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if usage === null}
					Counting the issues it is on…
				{:else if usage === 0}
					It is on no issues, so nothing else changes.
				{:else}
					It comes off {issueCount(usage)}. Nothing else about those issues changes.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if canMerge}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Merging into another label keeps the history. Deleting does not.
			</p>
		{/if}

		<AlertDialog.Footer>
			{#if canMerge}
				<Button
					variant="ghost"
					class="sm:mr-auto"
					disabled={removing}
					onclick={onmergeinstead}
				>
					Merge instead
				</Button>
			{/if}
			{#if removing}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action
				variant="destructive"
				disabled={removing || usage === null}
				onclick={onconfirm}
			>
				{removing ? "Deleting" : usage === null ? "Checking" : "Delete anyway"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
