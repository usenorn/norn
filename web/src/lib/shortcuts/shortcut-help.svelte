<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import { holdShortcuts, useShortcuts } from "./registry.svelte";
	import {
		displayKeys,
		isApplePlatform,
		shortcutGroupLabels,
		shortcutGroupOrder,
		type ShortcutGroup,
	} from "./shortcuts";

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const registry = useShortcuts();
	const apple = isApplePlatform();

	holdShortcuts(() => open);

	const groups = $derived(
		shortcutGroupOrder
			.map((group: ShortcutGroup) => ({
				group,
				label: shortcutGroupLabels[group],
				entries: registry.active.filter((shortcut) => shortcut.group === group),
			}))
			.filter((section) => section.entries.length > 0)
	);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>Keyboard shortcuts</Dialog.Title>
			<Dialog.Description>
				What this screen answers to right now. It changes as you move around.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex max-h-[60dvh] flex-col gap-5 overflow-y-auto">
			{#each groups as section (section.group)}
				<section class="flex flex-col gap-1.5">
					<h3 class="text-2xs font-medium tracking-wide text-muted-foreground uppercase">
						{section.label}
					</h3>
					<ul class="flex flex-col">
						{#each section.entries as shortcut (shortcut.id)}
							<li class="flex items-center justify-between gap-4 py-1.5">
								<span class="min-w-0 text-sm text-ink-900">{shortcut.label}</span>
								<span class="flex shrink-0 items-center gap-1">
									{#each shortcut.keys.slice(0, 1) as binding (binding)}
										<Kbd keys={displayKeys(binding, apple)} />
									{/each}
								</span>
							</li>
						{/each}
					</ul>
				</section>
			{:else}
				<p class="text-sm text-muted-foreground">
					Nothing on this screen answers to the keyboard yet.
				</p>
			{/each}
		</div>
	</Dialog.Content>
</Dialog.Root>
