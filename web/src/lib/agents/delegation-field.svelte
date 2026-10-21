<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { onDateAndTime } from "$lib/time";
	import { Button } from "$lib/components/ui/button/index.js";
	import { fieldTrigger } from "$lib/issues/issue-field.svelte";
	import type { DelegationPanel } from "./delegation";

	let {
		panel,
		editable,
		assigned,
		working,
		timezone,
		ondelegate,
		onrecall,
	}: {
		panel: DelegationPanel;
		editable: boolean;
		assigned: boolean;
		working: boolean;
		timezone: string;
		ondelegate: () => void;
		onrecall: () => void;
	} = $props();
</script>

<div class="relative flex min-h-7 items-start gap-1.5">
	<span
		class="mt-1.5 w-19.5 flex-none caption text-muted-foreground"
	>
		Delegated
	</span>
	<div class="flex min-h-7 min-w-0 flex-1 flex-col justify-center gap-0.5">
		{#if panel.kind === "loading"}
			<span class="mt-1 h-3 w-24 animate-breathe rounded-xs bg-paper-3" aria-hidden="true"></span>
		{:else if panel.kind === "unavailable"}
			<span class="py-0.5 text-md text-muted-foreground">Not available</span>
		{:else if panel.kind === "held"}
			<span class="flex min-w-0 items-center gap-1.75 py-0.5 text-md text-ink-900">
				<Bot class="size-icon-row flex-none text-muted-foreground" aria-hidden="true" />
				<span class="truncate">{panel.delegation.agentName}</span>
			</span>
			<span class="text-2xs text-muted-foreground">
				Since {onDateAndTime(panel.delegation.delegatedAt, timezone)}
			</span>
			{#if editable}
				<Button
					variant="ghost"
					size="sm"
					class="-ml-1.75 w-max px-1.75 text-2xs text-muted-foreground"
					disabled={working}
					onclick={onrecall}
				>
					Take it back
				</Button>
			{/if}
		{:else if editable && !assigned}
			<span class="py-0.5 text-md text-muted-foreground text-pretty">
				Assign it to somebody first. An agent takes this on for whoever it is assigned to.
			</span>
		{:else if editable}
			<button type="button" class={fieldTrigger()} disabled={working} onclick={ondelegate}>
				<Bot class="size-icon-row flex-none text-muted-foreground" aria-hidden="true" />
				<span class="truncate text-muted-foreground">Hand it to an agent</span>
			</button>
		{:else}
			<span class="py-0.5 text-md text-muted-foreground">Nobody</span>
		{/if}
	</div>
</div>
