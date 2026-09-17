<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { issueCount, labelFailureMessage, mergeTargets, type Label, type UsageRead } from "./labels";

	let {
		open = $bindable(false),
		label,
		labels,
		usage,
		removing,
		onconfirm,
		onretry,
		onmergeinstead,
	}: {
		open?: boolean;
		label: Label | null;
		labels: Label[];
		usage: UsageRead;
		removing: boolean;
		onconfirm: () => void;
		onretry: () => void;
		onmergeinstead: () => void;
	} = $props();

	const canMerge = $derived(label ? mergeTargets(label, labels).length > 0 : false);
	const inUse = $derived(usage.kind === "counted" && usage.issues > 0);
	const refused = $derived(usage.kind === "refused");
</script>

<AlertDialog.Root
	{open}
	onOpenChange={(next) => {
		if (!next && !removing) open = false;
	}}
>
	<AlertDialog.Content size="sm">
		<AlertDialog.Header>
			<Eyebrow tone={inUse || refused ? "danger" : "muted"}>
				{#if refused}
					Cannot check
				{:else if usage.kind === "counting"}
					Checking
				{:else if inUse}
					In use
				{:else}
					Not in use
				{/if}
			</Eyebrow>
			<AlertDialog.Title class="flex flex-wrap items-center gap-2">
				<span>Delete</span>
				{#if label}
					<Tag name={label.name} color={label.color} />
				{/if}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if usage.kind === "counting"}
					Counting the issues it is on…
				{:else if usage.kind === "refused"}
					{labelFailureMessage(usage.failure)} Until the count comes back, deleting stays unavailable,
					because the number has to be sent with the deletion.
				{:else if usage.issues === 0}
					It is on no issues, so nothing else changes.
				{:else}
					It comes off {issueCount(usage.issues)}. Nothing else about those issues changes.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if canMerge && !refused}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Merging into another label keeps the history. Deleting does not.
			</p>
		{/if}

		<AlertDialog.Footer>
			{#if canMerge}
				<Button variant="ghost" class="sm:mr-auto" disabled={removing} onclick={onmergeinstead}>
					Merge instead
				</Button>
			{/if}
			{#if removing}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			{#if refused}
				<Button disabled={removing} onclick={onretry}>Try again</Button>
			{:else}
				<AlertDialog.Action
					variant="destructive"
					disabled={removing || usage.kind !== "counted"}
					onclick={onconfirm}
				>
					{removing ? "Deleting" : usage.kind === "counted" ? "Delete anyway" : "Checking"}
				</AlertDialog.Action>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
