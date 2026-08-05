<script lang="ts" module>
	export type PickerOption = {
		value: string;
		label: string;
		href?: string;
		checked?: boolean;
		hint?: string;
		trailing?: boolean;
	};
</script>

<script lang="ts">
	import type { Snippet } from "svelte";
	import Check from "@lucide/svelte/icons/check";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import * as Command from "$lib/components/ui/command/index.js";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import { cn } from "$lib/utils.js";

	let {
		options,
		placeholder,
		open = $bindable(false),
		search = $bindable(""),
		align = "start",
		empty = "No matches",
		closeOnPick = true,
		onpick,
		trigger,
		mark,
		shortcut,
		class: className,
	}: {
		options: PickerOption[];
		placeholder: string;
		open?: boolean;
		search?: string;
		align?: "start" | "center" | "end";
		empty?: string;
		closeOnPick?: boolean;
		onpick?: (value: string) => void;
		trigger: Snippet<[Record<string, unknown>]>;
		mark?: Snippet<[PickerOption]>;
		shortcut?: Snippet;
		class?: string;
	} = $props();

	$effect(() => {
		if (!open) search = "";
	});
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			{@render trigger(props)}
		{/snippet}
	</Popover.Trigger>
	<Popover.Content {align} sideOffset={4} class={cn("w-51.5", className)}>
		<Command.Root>
			<Command.Input {placeholder} bind:value={search}>
				{#if shortcut}
					{@render shortcut()}
				{/if}
			</Command.Input>
			<Command.List>
				<Command.Empty>{empty}</Command.Empty>
				<Command.Group>
					{#each options as option (option.value)}
						{#snippet body()}
							{#if mark}
								<span class="inline-flex w-3.75 flex-none justify-center">
									{@render mark(option)}
								</span>
							{/if}
							<span class="min-w-0 flex-1 truncate">{option.label}</span>
							{#if option.hint}
								<span class="font-mono text-2xs text-muted-foreground">{option.hint}</span>
							{/if}
							{#if option.trailing}
								<ChevronRight class="text-muted-foreground" aria-hidden="true" />
							{:else if option.checked}
								<Check class="text-ink-900" aria-hidden="true" />
							{/if}
						{/snippet}

						{#if option.href}
							<Command.LinkItem
								href={option.href}
								value={option.label}
								onSelect={() => (open = false)}
							>
								{@render body()}
							</Command.LinkItem>
						{:else}
							<Command.Item
								value={option.label}
								onSelect={() => {
									if (closeOnPick) open = false;
									search = "";
									onpick?.(option.value);
								}}
							>
								{@render body()}
							</Command.Item>
						{/if}
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
