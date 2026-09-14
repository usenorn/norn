<script lang="ts">
	import SmilePlus from "@lucide/svelte/icons/smile-plus";
	import * as Command from "$lib/components/ui/command/index.js";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { emoji, matchingEmoji, type Emoji } from "$lib/editor/emoji";

	let {
		disabled = false,
		onpick,
	}: {
		disabled?: boolean;
		onpick: (glyph: string) => void;
	} = $props();

	let open = $state(false);
	let asked = $state("");

	const shown = $derived<Emoji[]>(asked.trim() === "" ? emoji : matchingEmoji(asked, 64));

	function pick(glyph: string) {
		open = false;
		asked = "";
		onpick(glyph);
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="ghost" size="icon-xs" aria-label="Emoji" {disabled}>
				<SmilePlus aria-hidden="true" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-72 p-0" align="start">
		<Command.Root shouldFilter={false}>
			<Command.Input placeholder="Search emoji" bind:value={asked} />
			<Command.List>
				{#if shown.length === 0}
					<p class="px-3 py-6 text-center text-sm text-muted-foreground">
						No emoji by that name.
					</p>
				{:else}
					<div class="grid grid-cols-8 gap-0.5 p-1">
						{#each shown as one (one.name)}
							<Button
								variant="ghost"
								size="glyph"
								aria-label={one.name}
								onclick={() => pick(one.glyph)}
							>
								{one.glyph}
							</Button>
						{/each}
					</div>
				{/if}
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
