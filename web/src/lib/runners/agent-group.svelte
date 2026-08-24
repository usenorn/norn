<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import CommandSnippet from "$lib/components/norn/command-snippet.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { agentPath } from "$lib/agents/agents";
	import CodebaseCard from "./codebase-card.svelte";
	import MachineRow from "./machine-row.svelte";
	import {
		installCommand,
		tokenPlaceholder,
		connectCommand,
		type AgentMachines,
		type Codebase,
		type Runner,
	} from "./runners";

	let {
		group,
		workspaceSlug,
		now,
		timezone,
		busyMachine,
		busyCodebase,
		copied,
		oncopy,
		onconnect,
		onpause,
		onresume,
		onrevoke,
		ondisconnect,
	}: {
		group: AgentMachines;
		workspaceSlug: string;
		now: string;
		timezone: string;
		busyMachine: string;
		busyCodebase: string;
		copied: string;
		oncopy: (text: string) => void;
		onconnect: () => void;
		onpause: (machine: Runner) => void;
		onresume: (machine: Runner) => void;
		onrevoke: (machine: Runner) => void;
		ondisconnect: (codebase: Codebase) => void;
	} = $props();

	const command = connectCommand(tokenPlaceholder);

	const live = $derived(group.machines.filter((machine) => machine.status !== "revoked"));
	const revoked = $derived(group.machines.filter((machine) => machine.status === "revoked"));
	const held = $derived(group.codebases.filter((codebase) => codebase.state !== "disconnected"));
	const released = $derived(group.codebases.filter((codebase) => codebase.state === "disconnected"));

	function machineOf(codebase: Codebase): Runner | undefined {
		return group.machines.find((machine) => machine.id === codebase.runnerId);
	}
</script>

<section class="flex min-w-0 flex-col gap-4 rounded-lg border border-line-subtle p-4">
	<div class="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
		<div class="flex min-w-0 items-center gap-2">
			<Bot class="size-icon-row flex-none text-muted-foreground" aria-hidden="true" />
			<a
				href={agentPath(workspaceSlug, group.agent.id)}
				class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900 motion-control hover:text-link"
			>
				{group.agent.name}
			</a>
			{#if group.agent.disabled}
				<Tag name="Disabled" class="flex-none" />
			{:else if live.length === 0}
				<Tag name="MCP only" class="flex-none" />
			{/if}
		</div>
		{#if group.agent.owner}
			<span class="flex-none text-xs text-muted-foreground">Owned by {group.agent.owner}</span>
		{/if}
	</div>

	{#if live.length === 0}
		<div class="flex min-w-0 flex-col gap-3">
			{#if group.agent.disabled}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{group.agent.name} is disabled, so nothing runs as it and no machine may connect to it.
					Enable it again on the agents screen first.
				</p>
			{:else}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{group.agent.name} has no machine, so it is MCP-only: it can read and change issues
					through an AI client, and there is nowhere for it to actually run code. Connecting a
					computer is what lets you delegate work that gets written, tested and put up for review.
				</p>
			{/if}
			<div class="flex min-w-0 flex-col gap-2">
				<Eyebrow>On the computer you want the work to happen on</Eyebrow>
				<CommandSnippet
					command={installCommand}
					label="Copy install command"
					copied={copied === installCommand}
					variant="ghost"
					{oncopy}
				/>
				<CommandSnippet
					{command}
					label="Copy connect command"
					copied={copied === command}
					variant="ghost"
					{oncopy}
				/>
			</div>
			<div>
				<Button variant="secondary" size="sm" onclick={onconnect}>Walk me through it</Button>
			</div>
		</div>
	{:else}
		<ul class="min-w-0 rounded-lg border border-line-subtle bg-paper-0">
			{#each live as machine (machine.id)}
				<MachineRow
					{machine}
					{now}
					{timezone}
					working={busyMachine === machine.id}
					{onpause}
					{onresume}
					{onrevoke}
				/>
			{/each}
		</ul>
	{/if}

	{#if revoked.length > 0}
		<div class="flex min-w-0 flex-col gap-2">
			<Eyebrow rule>Revoked</Eyebrow>
			<ul class="min-w-0 rounded-lg border border-line-subtle bg-paper-0">
				{#each revoked as machine (machine.id)}
					<MachineRow
						{machine}
						{now}
						{timezone}
						working={busyMachine === machine.id}
						{onpause}
						{onresume}
						{onrevoke}
					/>
				{/each}
			</ul>
		</div>
	{/if}

	{#if group.folders === "unreadable"}
		<Alert.Root variant="muted">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>Norn could not read this agent's folders</Alert.Title>
			<Alert.Description>
				The machines above are still listed. Reload in a moment to see what they hold.
			</Alert.Description>
		</Alert.Root>
	{:else if live.length > 0 && held.length === 0 && released.length === 0}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			No folder is connected to {group.agent.name} yet. Run
			<span class="font-mono text-xs text-ink-900">norn runner inspect</span>
			inside the folder you want it to work in, on that machine.
		</p>
	{/if}

	{#if held.length > 0}
		<div class="flex min-w-0 flex-col gap-2">
			<Eyebrow rule>Connected folders</Eyebrow>
			{#each held as codebase (codebase.id)}
				<CodebaseCard
					{codebase}
					machine={machineOf(codebase)}
					{timezone}
					working={busyCodebase === codebase.id}
					{copied}
					{oncopy}
					{ondisconnect}
				/>
			{/each}
		</div>
	{/if}

	{#if released.length > 0}
		<div class="flex min-w-0 flex-col gap-2">
			<Eyebrow rule>Disconnected folders</Eyebrow>
			{#each released as codebase (codebase.id)}
				<CodebaseCard
					{codebase}
					machine={machineOf(codebase)}
					{timezone}
					working={busyCodebase === codebase.id}
					{copied}
					{oncopy}
					{ondisconnect}
				/>
			{/each}
		</div>
	{/if}
</section>
