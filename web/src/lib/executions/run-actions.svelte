<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { onDateAndTime } from "$lib/time";
	import {
		canCancel,
		canRestart,
		canRetain,
		retentionClock,
		retentionLine,
		shouldShowRetention,
		type Execution,
	} from "./executions";

	let {
		execution,
		working,
		now,
		timezone,
		oncancel,
		onrestart,
		onretain,
	}: {
		execution: Execution;
		working: boolean;
		now: string;
		timezone: string;
		oncancel: () => void;
		onrestart: () => void;
		onretain: () => void;
	} = $props();

	let confirming = $state(false);

	const clock = $derived(retentionClock(execution, now));

	const deadline = $derived(
		clock.kind === "holding" ? clock.until : clock.kind === "given_back" ? clock.at : now
	);
</script>

<div class="flex min-w-0 flex-col gap-1.5">
	<div class="flex flex-wrap items-center gap-2">
		{#if canRestart(execution)}
			<Button size="sm" disabled={working} onclick={onrestart}>Run it again</Button>
		{/if}

		{#if canRetain(execution)}
			<Button variant="secondary" size="sm" disabled={working} onclick={onretain}>
				Keep it an hour longer
			</Button>
		{/if}

		{#if canCancel(execution)}
			{#if confirming}
				<span class="text-xs text-muted-foreground">Stop this run?</span>
				<Button
					variant="destructive"
					size="sm"
					disabled={working}
					onclick={() => {
						confirming = false;
						oncancel();
					}}
				>
					Stop it
				</Button>
				<Button variant="ghost" size="sm" disabled={working} onclick={() => (confirming = false)}>
					Leave it running
				</Button>
			{:else}
				<Button variant="secondary" size="sm" disabled={working} onclick={() => (confirming = true)}>
					Stop this run
				</Button>
			{/if}
		{/if}
	</div>

	{#if shouldShowRetention(execution)}
		<p class="text-2xs text-muted-foreground">
			{retentionLine(clock, onDateAndTime(deadline, timezone))}
		</p>
	{/if}
</div>
