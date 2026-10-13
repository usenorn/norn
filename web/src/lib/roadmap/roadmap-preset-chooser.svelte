<script lang="ts">
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import SettingsSectionHeader from "$lib/settings/settings-section-header.svelte";
	import {
		chosenPresets,
		presetCopy,
		type ChosenPreset,
		type RoadmapActivity,
		type RoadmapEntry,
		type RoadmapPreset,
	} from "./admin";

	let {
		preset,
		intended,
		conflicts,
		activity,
		onchoose,
		onremove,
	}: {
		preset: RoadmapPreset;
		intended: ChosenPreset | undefined;
		conflicts: RoadmapEntry[];
		activity: RoadmapActivity;
		onchoose: (preset: ChosenPreset) => void;
		onremove: (entry: RoadmapEntry) => void;
	} = $props();

	const busy = $derived(activity.kind !== "idle");
	const selected = $derived(activity.kind === "choosing_preset" ? activity.preset : (intended ?? ""));

	function isChosen(value: string): value is ChosenPreset {
		return chosenPresets.some((chosen) => chosen === value);
	}
</script>

<section class="flex flex-col gap-3" aria-labelledby="roadmap-preset">
	<SettingsSectionHeader id="roadmap-preset" title="What to share">
		{#if preset === "custom" && intended}
			Some entries do not fit this preset. Nothing is removed for you; remove them yourself or pick another preset.
		{:else if preset === "custom"}
			The selection fits no single preset. Nothing is removed for you; pick a preset to see what does not fit.
		{:else if preset === "unstarted"}
			Pick how much of your planning the public roadmap shows. You can change it later.
		{:else}
			A preset only decides what you can add and whether selected issues are listed. Descriptions, dates, status
			and progress stay as you set them.
		{/if}
	</SettingsSectionHeader>

	<RadioGroup.Root
		value={selected}
		onValueChange={(value) => {
			if (isChosen(value) && value !== intended) onchoose(value);
		}}
		disabled={busy}
		aria-label="What to share"
		class="grid gap-2 sm:grid-cols-3"
	>
		{#each chosenPresets as option (option)}
			<RadioGroup.Item variant="card" value={option}>
				<span>{presetCopy[option].title}</span>
				<small>{presetCopy[option].hint}</small>
			</RadioGroup.Item>
		{/each}
	</RadioGroup.Root>

	{#if intended && conflicts.length > 0}
		<Alert.Root variant="muted">
			<Alert.Title>
				{conflicts.length === 1 ? "1 entry does not fit" : `${conflicts.length} entries do not fit`}
				{presetCopy[intended].title}
			</Alert.Title>
			<Alert.Description>
				<span class="block">They stay on the roadmap until you remove them.</span>
				<ul class="mt-2 flex flex-col gap-1">
					{#each conflicts as entry (entry.id)}
						<li class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
							<span class="min-w-0 break-words text-ink-900">{entry.publicTitle || entry.source?.label}</span>
							<Button size="xs" variant="ghost" disabled={busy} onclick={() => onremove(entry)}>Remove</Button>
						</li>
					{/each}
				</ul>
			</Alert.Description>
		</Alert.Root>
	{/if}
</section>
