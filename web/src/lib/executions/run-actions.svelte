<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { canCancel, canRestart, canRetain, type Execution } from "./executions";

	let {
		execution,
		working,
		oncancel,
		onrestart,
		onretain,
	}: {
		execution: Execution;
		working: boolean;
		oncancel: () => void;
		onrestart: () => void;
		onretain: () => void;
	} = $props();

	let confirming = $state(false);
</script>

<div class="flex flex-wrap items-center gap-2">
	{#if canRestart(execution)}
		<Button size="sm" disabled={working} onclick={onrestart}>Run it again</Button>
	{/if}

	{#if canRetain(execution)}
		<Button variant="secondary" size="sm" disabled={working} onclick={onretain}>
			Keep the preview an hour longer
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
