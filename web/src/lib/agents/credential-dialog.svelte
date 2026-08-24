<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import Copy from "@lucide/svelte/icons/copy";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import CommandSnippet from "$lib/components/norn/command-snippet.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { mcpAddCommand, mcpServerUrl, type Agent } from "./agents";

	let {
		open,
		agent,
		value,
		origin,
		workspaceName,
		rotated = false,
		onclose,
	}: {
		open: boolean;
		agent: Agent | null;
		value: string;
		origin: string;
		workspaceName: string;
		rotated?: boolean;
		onclose: () => void;
	} = $props();

	let copied = $state("");

	const command = $derived(agent ? mcpAddCommand(agent.name, origin, value) : "");
	const serverUrl = $derived(mcpServerUrl(origin));

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
		copied = text;
	}
</script>

<Dialog.Root {open} onOpenChange={(next) => !next && onclose()}>
	<Dialog.Content class="sm:max-w-140">
		<Dialog.Header>
			<Dialog.Title>
				{rotated ? `${agent?.name}'s new credential` : `Copy ${agent?.name}'s credential now`}
			</Dialog.Title>
			<Dialog.Description>
				Norn keeps only a hash of this, so it cannot be shown again. Copy it before you close
				this{rotated ? ". The credential it replaces has stopped working" : ""}.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex min-w-0 flex-col gap-2">
			<Eyebrow class="text-ink-600">Credential</Eyebrow>
			<p
				class="min-w-0 rounded-md bg-paper-2 p-3 font-mono text-xs break-all text-ink-900"
				data-testid="agent-credential"
			>
				{value}
			</p>
			<Button variant="secondary" size="sm" class="w-max" onclick={() => copy(value)}>
				{#if copied === value}
					<Check aria-hidden="true" />
					Copied
				{:else}
					<Copy aria-hidden="true" />
					Copy credential
				{/if}
			</Button>
		</div>

		<div class="flex min-w-0 flex-col gap-2 border-t border-line-subtle pt-4">
			<Eyebrow class="text-ink-600">Connect an AI client to {workspaceName}</Eyebrow>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				This is the whole setup. Everything the client does arrives as {agent?.name}, bounded by
				the permissions you chose, and stops the moment you disable it.
			</p>
			<CommandSnippet {command} label="Copy command" copied={copied === command} oncopy={copy} />
			<Button variant="ghost" size="sm" class="w-max" onclick={() => copy(serverUrl)}>
				{copied === serverUrl ? "Copied" : "Copy server URL"}
			</Button>
		</div>

		<Dialog.Footer>
			<Button onclick={onclose}>I have copied it</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
