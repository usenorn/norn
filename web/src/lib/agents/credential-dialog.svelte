<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import Copy from "@lucide/svelte/icons/copy";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
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
		inline = false,
		doneHref,
		onclose,
	}: {
		open: boolean;
		agent: Agent | null;
		value: string;
		origin: string;
		workspaceName: string;
		rotated?: boolean;
		inline?: boolean;
		doneHref?: string;
		onclose: () => void;
	} = $props();

	type ClipboardState =
		| { kind: "idle" }
		| { kind: "copying"; value: string }
		| { kind: "copied"; value: string }
		| { kind: "unavailable" };

	let clipboard = $state.raw<ClipboardState>({ kind: "idle" });
	let copySequence = 0;

	const command = $derived(agent ? mcpAddCommand(agent.name, origin, value) : "");
	const serverUrl = $derived(mcpServerUrl(origin));
	const title = $derived(
		rotated ? `${agent?.name}'s new credential` : `Copy ${agent?.name}'s credential now`
	);
	const description = $derived(
		`Norn keeps only a hash of this, so it cannot be shown again. Copy it before you close this${rotated ? ". The credential it replaces has stopped working" : ""}.`
	);

	async function copy(text: string) {
		const sequence = ++copySequence;
		clipboard = { kind: "copying", value: text };

		try {
			await navigator.clipboard.writeText(text);
			if (!open || sequence !== copySequence) return;
			clipboard = { kind: "copied", value: text };
		} catch {
			if (!open || sequence !== copySequence) return;
			clipboard = { kind: "unavailable" };
		}
	}

	$effect(() => {
		if (!open) {
			copySequence += 1;
			clipboard = { kind: "idle" };
		}
	});
</script>

{#snippet credential()}
		{#if clipboard.kind === "unavailable"}
			<Alert.Root variant="destructive">
				<TriangleAlert aria-hidden="true" />
				<Alert.Title>Could not copy</Alert.Title>
				<Alert.Description>Select the credential and copy it manually.</Alert.Description>
			</Alert.Root>
		{/if}

		<div class="flex min-w-0 flex-col gap-2">
			<Eyebrow class="text-ink-600">Credential</Eyebrow>
			<Input
				data-testid="agent-credential"
				value={value}
				readonly
				aria-label="Agent credential"
				autocomplete="off"
				spellcheck="false"
			/>
			<Button
				variant="secondary"
				size="sm"
				class="w-max"
				disabled={clipboard.kind === "copying"}
				onclick={() => copy(value)}
			>
				{#if clipboard.kind === "copied" && clipboard.value === value}
					<Check aria-hidden="true" />
					Copied
				{:else if clipboard.kind === "copying" && clipboard.value === value}
					Copying…
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
			<CommandSnippet
				{command}
				label="Copy command"
				copied={clipboard.kind === "copied" && clipboard.value === command}
				oncopy={copy}
			/>
			<Button
				variant="ghost"
				size="sm"
				class="w-max"
				disabled={clipboard.kind === "copying"}
				onclick={() => copy(serverUrl)}
			>
				{clipboard.kind === "copied" && clipboard.value === serverUrl
					? "Copied"
					: "Copy server URL"}
			</Button>
		</div>

		<div class="flex justify-end">
			<Button href={doneHref} onclick={onclose}>I have copied it</Button>
		</div>
	{/snippet}

{#if inline}
	<section
		class="notch flex flex-col gap-6 p-6"
		aria-labelledby="agent-credential-title"
		aria-describedby="agent-credential-description"
	>
		<header class="flex flex-col gap-1.5">
			<h2 id="agent-credential-title" class="text-lg font-medium tracking-snug text-ink-900">
				{title}
			</h2>
			<p id="agent-credential-description" class="text-sm leading-normal text-muted-foreground">
				{description}
			</p>
		</header>
		{@render credential()}
	</section>
{:else}
	<Dialog.Root {open} onOpenChange={(next) => !next && onclose()}>
		<Dialog.Content class="sm:max-w-140">
			<Dialog.Header>
				<Dialog.Title>{title}</Dialog.Title>
				<Dialog.Description>{description}</Dialog.Description>
			</Dialog.Header>
			{@render credential()}
	</Dialog.Content>
	</Dialog.Root>
{/if}
