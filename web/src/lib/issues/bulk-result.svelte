<script lang="ts">
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { failures, outcomeLabel, settled, summary, type BulkActionResult } from "$lib/issues/bulk";

	let {
		result,
		unreadable = false,
		working = false,
		onretry,
		onretryfailed,
	}: {
		result: BulkActionResult;
		unreadable?: boolean;
		working?: boolean;
		onretry?: () => void;
		onretryfailed?: () => void;
	} = $props();

	const running = $derived(!settled(result.status) && !unreadable);
	const notApplied = $derived(failures(result));
	const progress = $derived({
		notStarted: Math.max((result.expected ?? result.processed) - result.processed, 0),
		active: 0,
		complete: result.processed,
		abandoned: 0,
	});
</script>

<Alert.Root variant={(notApplied.length > 0 || unreadable) && !running ? "destructive" : "default"}>
	{#if running}
		<CircleCheck aria-hidden="true" />
		<Alert.Title>Working through the selection</Alert.Title>
	{:else if unreadable}
		<CircleX aria-hidden="true" />
		<Alert.Title>We lost sight of this change</Alert.Title>
	{:else if notApplied.length > 0}
		<CircleX aria-hidden="true" />
		<Alert.Title>Some issues did not change</Alert.Title>
	{:else}
		<CircleCheck aria-hidden="true" />
		<Alert.Title>Done</Alert.Title>
	{/if}

	<Alert.Description>
		<span class="block">
			{unreadable
				? "It may still be running. Nothing was undone, and asking again is safe."
				: summary(result)}
		</span>

		{#if running}
			<span class="mt-2 block">
				<ProgressBar progress={progress} label={false} />
				{#if result.expected == null}
					<span class="font-mono text-xs text-muted-foreground">
						This set is still being worked out, so there is no finish line to show yet.
					</span>
				{/if}
			</span>
		{/if}

		{#if notApplied.length > 0}
			<ul class="mt-2 flex flex-col gap-0.5">
				{#each notApplied as outcome (outcome.issueId)}
					<li class="flex items-baseline gap-2 text-sm">
						<span class="font-mono text-xs">{outcome.reference || "an issue"}</span>
						<span class="text-muted-foreground">{outcomeLabel(outcome.outcome)}</span>
					</li>
				{/each}
			</ul>
		{/if}

		{#if unreadable && onretry}
			<span class="mt-2 block">
				<Button variant="secondary" size="sm" disabled={working} onclick={onretry}>
					Check again
				</Button>
			</span>
		{:else if !running && notApplied.length > 0 && onretryfailed}
			<span class="mt-2 block">
				<Button variant="secondary" size="sm" disabled={working} onclick={onretryfailed}>
					Try the {notApplied.length === 1 ? "one" : `${notApplied.length}`} that did not change
				</Button>
			</span>
		{/if}
	</Alert.Description>
</Alert.Root>
