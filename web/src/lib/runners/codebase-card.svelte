<script lang="ts">
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import CommandSnippet from "$lib/components/norn/command-snippet.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { onDateAndTime } from "$lib/time";
	import FolderState from "./folder-state.svelte";
	import {
		gatewayLabels,
		reconfirmCommand,
		remoteLabel,
		repositoryPath,
		runtimeLabels,
		toolLabel,
		type Codebase,
		type Runner,
	} from "./runners";

	let {
		codebase,
		machine,
		timezone,
		working,
		copied,
		oncopy,
		ondisconnect,
	}: {
		codebase: Codebase;
		machine: Runner | undefined;
		timezone: string;
		working: boolean;
		copied: string;
		oncopy: (text: string) => void;
		ondisconnect: (codebase: Codebase) => void;
	} = $props();

	const on = $derived(machine?.name ?? "a machine that is no longer listed");
</script>

<article class="flex min-w-0 flex-col gap-3 rounded-lg border border-line-subtle bg-paper-0 p-4">
	<div class="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
		<div class="flex min-w-0 flex-col gap-0.5">
			<h4 class="min-w-0 truncate text-sm font-medium text-ink-900">{codebase.name}</h4>
			<p class="min-w-0 truncate font-mono text-2xs text-muted-foreground" title={codebase.rootPath}>
				{codebase.rootPath}
			</p>
		</div>
		<FolderState state={codebase.state} class="flex-none" />
	</div>

	<p class="text-xs leading-normal text-muted-foreground text-pretty">
		On {on} · connected {onDateAndTime(codebase.connectedAt, timezone)}
	</p>

	{#if codebase.state === "drift"}
		<Alert.Root variant="warning">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>The repositories in this folder have changed</Alert.Title>
			<Alert.Description>
				Norn holds what the machine last reported, and nothing is blocked by this — runs carry on
				against the repositories they already know. Only the machine itself can agree the new set,
				so re-confirm it there:
			</Alert.Description>
			<Alert.Action class="min-w-0">
				<CommandSnippet
					command={reconfirmCommand}
					label="Copy command"
					copied={copied === reconfirmCommand}
					variant="ghost"
					{oncopy}
				/>
			</Alert.Action>
		</Alert.Root>
	{/if}

	{#if codebase.repositories.length > 0}
		<div class="flex min-w-0 flex-col gap-1.5">
			<Eyebrow rule>Repositories</Eyebrow>
			<ul class="flex min-w-0 flex-col gap-1.5">
				{#each codebase.repositories as repository (repository.relPath)}
					<li class="flex min-w-0 flex-col gap-0.5">
						<div class="flex min-w-0 items-baseline gap-2">
							<span class="min-w-0 truncate text-xs text-ink-900">{repository.name}</span>
							{#if repository.defaultBranch}
								<span
									class="inline-flex flex-none items-center gap-1 font-mono text-2xs text-muted-foreground"
								>
									<GitBranch class="size-3" aria-hidden="true" />
									{repository.defaultBranch}
								</span>
							{/if}
						</div>
						<span class="min-w-0 truncate font-mono text-2xs text-muted-foreground">
							{repositoryPath(repository)}{remoteLabel(repository.remote)
								? ` · ${remoteLabel(repository.remote)}`
								: ""}
						</span>
					</li>
				{/each}
			</ul>
		</div>
	{:else}
		<p class="text-xs leading-normal text-muted-foreground text-pretty">
			This folder holds no repositories the machine could read.
		</p>
	{/if}

	<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
		<div class="flex min-w-0 flex-col gap-0.5">
			<Eyebrow>Shared files</Eyebrow>
			<span class="min-w-0 text-xs break-words text-ink-900">
				{codebase.sharedFiles.length > 0 ? codebase.sharedFiles.join(", ") : "None found"}
			</span>
		</div>
		<div class="flex min-w-0 flex-col gap-0.5">
			<Eyebrow>Runtimes</Eyebrow>
			<span class="min-w-0 text-xs break-words text-ink-900">
				{codebase.runtimes.length > 0
					? codebase.runtimes.map((runtime) => runtimeLabels[runtime]).join(", ")
					: "None detected"}
			</span>
		</div>
		<div class="flex min-w-0 flex-col gap-0.5">
			<Eyebrow>Coding agents</Eyebrow>
			<span class="min-w-0 text-xs break-words text-ink-900">
				{codebase.tools.length > 0 ? codebase.tools.map(toolLabel).join(", ") : "None detected"}
			</span>
		</div>
	</div>

	{#if codebase.previewGateway}
		<p class="text-xs leading-normal text-muted-foreground text-pretty">
			{gatewayLabels[codebase.previewGateway]}
		</p>
	{/if}

	{#if codebase.state !== "disconnected"}
		<div>
			<Button variant="ghost" size="sm" disabled={working} onclick={() => ondisconnect(codebase)}>
				{working ? "Disconnecting" : "Disconnect this folder"}
			</Button>
		</div>
	{/if}
</article>
