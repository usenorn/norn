<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { onDateAndTime } from "$lib/time";
	import {
		channelLabels,
		onlyTimeLimitHolds,
		verdictLabels,
		verdictTones,
		type CheckEvidence,
	} from "./checks";

	let {
		record,
		timezone,
	}: {
		record: CheckEvidence;
		timezone: string;
	} = $props();

	const who = $derived(record.actorName || (record.actorKind === "system" ? "Norn" : "Someone"));
	const drifted = $derived(
		Math.abs(new Date(record.receivedAt).getTime() - new Date(record.observedAt).getTime()) >
			60_000
	);
</script>

<li
	class="flex min-w-0 flex-col gap-1.5 border-l-2 py-0.5 pl-2.5 {record.expired
		? 'border-line-subtle opacity-70'
		: 'border-line-default'}"
>
	<div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
		<span class="text-sm font-medium {verdictTones[record.verdict]}">
			{verdictLabels[record.verdict]}
		</span>
		<span class="text-xs text-muted-foreground">{channelLabels[record.channel]}</span>
		<span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">
			{who} ({record.actorKind})
		</span>
		<span class="text-xs text-muted-foreground">
			{onDateAndTime(record.receivedAt, timezone)}
		</span>
	</div>

	{#if record.command}
		<pre
			class="min-w-0 overflow-x-auto rounded-sm bg-paper-2 px-2 py-1 font-mono text-2xs text-ink-600">{record.command}</pre>
	{/if}

	<pre
		class="max-h-64 min-w-0 overflow-auto rounded-sm bg-paper-2 px-2 py-1.5 font-mono text-2xs leading-normal text-ink-900">{record.output}</pre>

	<div class="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
		{#if record.exitCode !== undefined}
			<span class="font-mono text-2xs text-muted-foreground">exit {record.exitCode}</span>
		{/if}
		{#if record.redactions > 0}
			<span class="text-2xs text-warning">
				Norn removed {record.redactions}
				{record.redactions === 1 ? "secret" : "secrets"} before storing this
			</span>
		{/if}
		{#if record.truncated}
			<span class="text-2xs text-muted-foreground">
				Longer than Norn keeps; the middle was dropped
			</span>
		{/if}
		{#if drifted}
			<span class="text-2xs text-muted-foreground">
				Observed {onDateAndTime(record.observedAt, timezone)}
			</span>
		{/if}
		{#if record.commitSha}
			<span class="font-mono text-2xs text-muted-foreground">
				at {record.commitSha.slice(0, 7)}
			</span>
		{/if}
	</div>

	{#if record.expired}
		<p class="text-2xs leading-normal text-muted-foreground text-pretty">
			This no longer counts towards proving the criterion{record.expiryReason === "time_limit"
				? ", because it is older than the time limit"
				: ""}.
		</p>
	{:else if onlyTimeLimitHolds(record)}
		<p class="text-2xs leading-normal text-muted-foreground text-pretty">
			Nothing here is linked to a change, so only the time limit can take this proof away.
		</p>
	{/if}
</li>

