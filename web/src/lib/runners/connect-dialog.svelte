<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { api } from "$lib/api";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import CommandSnippet from "$lib/components/norn/command-snippet.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import type { WorkspaceAgent } from "$lib/agents/agents";
	import { connectCommand, installCommand, tokenPlaceholder } from "./runners";

	let {
		open = $bindable(false),
		workspaceId,
		agents,
		copied,
		oncopy,
	}: {
		open?: boolean;
		workspaceId: string;
		agents: WorkspaceAgent[];
		copied: string;
		oncopy: (text: string) => void;
	} = $props();

	type Minted = { agentId: string; value?: string; refused?: string };

	let chosenId = $state("");
	let issuing = $state(false);
	let minted = $state.raw<Minted | null>(null);

	const chosen = $derived(agents.find((entry) => entry.agent.id === chosenId));
	const settled = $derived(minted?.agentId === chosenId ? minted : null);
	const issued = $derived(settled?.value ?? "");
	const refused = $derived(settled?.refused ?? "");
	const command = $derived(connectCommand(issued || tokenPlaceholder));

	function forget(next: boolean) {
		if (!next) minted = null;
	}

	async function issue() {
		if (!chosen) return;

		const agentId = chosen.agent.id;

		issuing = true;
		minted = null;

		try {
			const { data, error } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/credential",
				{ params: { path: { workspaceId, agentId } } }
			);

			if (error || !data) {
				minted = {
					agentId,
					refused:
						error?.status === 403
							? "You may not issue a token for this agent. Ask an administrator of this workspace."
							: "Norn could not issue a token. Wait a moment and try again.",
				};

				return;
			}

			minted = { agentId, value: data.value };
		} catch {
			minted = { agentId, refused: "Norn could not issue a token. Wait a moment and try again." };
		} finally {
			issuing = false;
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={forget}>
	<Dialog.Content class="sm:max-w-140">
		<Dialog.Header>
			<Dialog.Title>Connect a machine</Dialog.Title>
			<Dialog.Description>
				A machine acts as one of your agents. Run these two commands on the computer you want the
				work to happen on — not here.
			</Dialog.Description>
		</Dialog.Header>

		{#if agents.length === 0}
			<p class="text-md leading-normal text-muted-foreground text-pretty">
				No agents are registered in this workspace yet. Register one first, and a machine can act
				as it.
			</p>
		{:else}
			<div class="flex min-w-0 flex-col gap-2">
				<Eyebrow class="text-ink-600">Which agent this machine acts as</Eyebrow>
				<Select.Root type="single" value={chosenId} onValueChange={(value) => (chosenId = value)}>
					<Select.Trigger>{chosen ? chosen.agent.name : "Choose an agent"}</Select.Trigger>
					<Select.Content>
						{#each agents as entry (entry.agent.id)}
							<Select.Item value={entry.agent.id} label={entry.agent.name}>
								<Bot class="size-3.5 text-muted-foreground" aria-hidden="true" />
								{entry.agent.name}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Everything this machine does arrives as that agent, bounded by the permissions it
					already has.
				</p>
			</div>

			<div class="flex min-w-0 flex-col gap-2 border-t border-line-subtle pt-4">
				<Eyebrow class="text-ink-600">1 · Install the runner</Eyebrow>
				<CommandSnippet
					command={installCommand}
					label="Copy command"
					copied={copied === installCommand}
					{oncopy}
				/>
			</div>

			<div class="flex min-w-0 flex-col gap-2 border-t border-line-subtle pt-4">
				<Eyebrow class="text-ink-600">2 · Bind it to {chosen?.agent.name ?? "the agent"}</Eyebrow>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					The token is the agent's ordinary API token — the same one that goes into an MCP config.
					There is nothing else to mint. It is read once and thrown away: what the machine keeps
					afterwards is a credential of its own, bound to a key it generated, which never leaves
					that computer.
				</p>
				<CommandSnippet
					{command}
					label="Copy command"
					copied={copied === command}
					{oncopy}
				/>
				{#if issued}
					<p class="text-sm leading-normal text-warning text-pretty" role="status">
						This is {chosen?.agent.name}'s new token, and Norn keeps only a hash of it — copy the
						command before you close this. The token it replaces has stopped working, so anything
						already using it needs the new one.
					</p>
				{:else}
					<div class="flex flex-wrap items-center gap-2">
						<Button variant="ghost" size="sm" disabled={!chosen || issuing} onclick={issue}>
							{issuing ? "Issuing" : "Issue a new token"}
						</Button>
						<span class="text-sm text-muted-foreground">
							Only if you no longer have it — a new one stops the old one working.
						</span>
					</div>
				{/if}
				{#if refused}
					<p class="text-sm leading-normal text-destructive text-pretty" role="alert">
						{refused}
					</p>
				{/if}
			</div>

			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				The machine appears here within a few seconds of connecting. Connect a folder to it
				separately, by running <span class="font-mono text-xs text-ink-900">norn runner inspect</span>
				inside the folder you want it to work in.
			</p>
		{/if}

		<Dialog.Footer>
			<Button onclick={() => (open = false)}>Done</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
