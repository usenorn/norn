<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import MachineState from "./machine-state.svelte";
	import {
		hostLine,
		machineStanding,
		seenLine,
		slotLine,
		standingDetail,
		versionLine,
		type Runner,
	} from "./runners";

	let {
		machine,
		now,
		timezone,
		working,
		onpause,
		onresume,
		onrevoke,
	}: {
		machine: Runner;
		now: string;
		timezone: string;
		working: boolean;
		onpause: (machine: Runner) => void;
		onresume: (machine: Runner) => void;
		onrevoke: (machine: Runner) => void;
	} = $props();

	let confirming = $state(false);

	const standing = $derived(machineStanding(machine));
	const detail = $derived(standingDetail(machine));
	const revoked = $derived(machine.status === "revoked");

	const facts = $derived([
		{ label: "Host", value: machine.host.hostname },
		{ label: "Platform", value: hostLine(machine.host) },
		{ label: "Version", value: versionLine(machine.host) },
		{ label: "Slots", value: slotLine(machine) },
	]);
</script>

<li class="flex min-w-0 flex-col gap-3 border-b border-line-subtle p-4 last:border-b-0">
	<div class="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
		<h4 class="min-w-0 truncate text-sm font-medium text-ink-900">{machine.name}</h4>
		<MachineState {machine} class="flex-none" />
	</div>

	<dl class="grid grid-cols-2 gap-x-4 gap-y-2.5 sm:grid-cols-4">
		{#each facts as fact (fact.label)}
			<div class="flex min-w-0 flex-col gap-0.5">
				<dt><Eyebrow>{fact.label}</Eyebrow></dt>
				<dd class="truncate text-xs text-ink-900" title={fact.value}>{fact.value}</dd>
			</div>
		{/each}
	</dl>

	<p class="text-xs leading-normal text-muted-foreground text-pretty">
		{seenLine(machine, now, timezone)}{detail ? ` · ${detail}` : ""}
	</p>

	{#if !revoked}
		<div class="flex flex-wrap gap-2">
			{#if standing === "paused"}
				<Button variant="secondary" size="sm" disabled={working} onclick={() => onresume(machine)}>
					{working ? "Starting" : "Take work again"}
				</Button>
			{:else}
				<Button variant="secondary" size="sm" disabled={working} onclick={() => onpause(machine)}>
					{working ? "Pausing" : "Stop taking work"}
				</Button>
			{/if}
			<Button variant="ghost" size="sm" disabled={working} onclick={() => (confirming = true)}>
				Revoke
			</Button>
		</div>
	{/if}
</li>

<AlertDialog.Root bind:open={confirming}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke {machine.name}?</AlertDialog.Title>
			<AlertDialog.Description>
				This cuts {machine.name} off and cannot be undone — the machine has to be connected again
				from scratch. It does not disable {machine.agentName}: the agent keeps working on its other
				machines and over MCP. To stop this machine taking work for a while instead, pause it.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Keep it</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={() => onrevoke(machine)}>
				Revoke this machine
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
