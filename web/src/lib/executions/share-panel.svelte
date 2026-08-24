<script lang="ts">
	import Copy from "@lucide/svelte/icons/copy";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { onDateAndTime } from "$lib/time";
	import {
		noShareLinksLine,
		shareLifetimes,
		shareOnceLine,
		shareStanding,
		shareStandingLabel,
		shareUseLine,
		type ExecutionPreview,
		type PreviewShareLink,
	} from "./executions";

	let {
		preview,
		links,
		minted,
		working,
		now,
		timezone,
		onshare,
		onrevoke,
	}: {
		preview: ExecutionPreview;
		links: PreviewShareLink[];
		minted?: string;
		working: boolean;
		now: string;
		timezone: string;
		onshare: (name: string, lifetimeSeconds: number, passcode: string) => void;
		onrevoke: (name: string, linkId: string) => void;
	} = $props();

	let minting = $state(false);
	let lifetime = $state(String(shareLifetimes[0].seconds));
	let passcode = $state("");
	let copied = $state(false);

	const chosen = $derived(
		shareLifetimes.find((held) => String(held.seconds) === lifetime)?.label ?? "How long"
	);

	const tones: Record<string, string> = {
		live: "text-success",
		expired: "text-muted-foreground",
		revoked: "text-muted-foreground",
	};

	async function copy() {
		if (!minted) return;

		await navigator.clipboard.writeText(minted);
		copied = true;
	}

	function mint() {
		minting = false;
		onshare(preview.name, Number(lifetime), passcode.trim());
		passcode = "";
	}
</script>

<div class="flex min-w-0 flex-col gap-1.5">
	{#if links.length === 0}
		<p class="text-2xs text-muted-foreground">{noShareLinksLine}</p>
	{:else}
		<ul class="flex min-w-0 flex-col gap-1">
			{#each links as link (link.id)}
				{@const standing = shareStanding(link, now)}
				<li class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class={`text-2xs ${tones[standing]}`}>{shareStandingLabel(standing)}</span>
					{#if standing === "live"}
						<span class="text-2xs text-muted-foreground">
							until {onDateAndTime(link.expiresAt, timezone)}
						</span>
					{/if}
					<span class="text-2xs text-muted-foreground">{shareUseLine(link)}</span>
					{#if link.needsPasscode}
						<span class="text-2xs text-muted-foreground">· passcode</span>
					{/if}
					<span class="flex-1"></span>
					{#if standing === "live"}
						<Button
							variant="ghost"
							size="sm"
							disabled={working}
							onclick={() => onrevoke(preview.name, link.id)}
						>
							Revoke
						</Button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if minted}
		<div class="flex min-w-0 flex-col gap-1 rounded-sm border border-line-subtle p-2">
			<div class="flex items-baseline justify-between gap-2">
				<span class="min-w-0 font-mono text-2xs break-all text-ink-900">{minted}</span>
				<Button variant="ghost" size="sm" onclick={copy}>
					<Copy aria-hidden="true" />
					{copied ? "Copied" : "Copy"}
				</Button>
			</div>
			<p class="text-2xs text-muted-foreground">{shareOnceLine}</p>
		</div>
	{/if}

	{#if minting}
		<div class="flex min-w-0 flex-wrap items-end gap-2">
			<div class="flex flex-col gap-1">
				<Label for={`lifetime-${preview.id}`} class="text-2xs">Lasts</Label>
				<Select.Root type="single" bind:value={lifetime}>
					<Select.Trigger id={`lifetime-${preview.id}`} class="w-32">{chosen}</Select.Trigger>
					<Select.Content>
						{#each shareLifetimes as choice (choice.seconds)}
							<Select.Item value={String(choice.seconds)}>{choice.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="flex flex-col gap-1">
				<Label for={`passcode-${preview.id}`} class="text-2xs">Passcode, if you want one</Label>
				<Input
					id={`passcode-${preview.id}`}
					bind:value={passcode}
					minlength={6}
					maxlength={128}
					autocomplete="off"
					class="w-48"
				/>
			</div>

			<Button
				size="sm"
				disabled={working || (passcode.trim() !== "" && passcode.trim().length < 6)}
				onclick={mint}
			>
				Make the link
			</Button>
			<Button variant="ghost" size="sm" disabled={working} onclick={() => (minting = false)}>
				Cancel
			</Button>
		</div>
	{:else}
		<div>
			<Button variant="ghost" size="sm" disabled={working} onclick={() => (minting = true)}>
				Share this preview
			</Button>
		</div>
	{/if}
</div>
