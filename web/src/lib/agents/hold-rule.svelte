<script lang="ts">
	import * as Select from "$lib/components/ui/select/index.js";
	import { holdOptionHints, holdOptionLabels, type AgentHold } from "./agents";

	let {
		title,
		detail,
		options,
		value,
		disabled,
		onchange,
	}: {
		title: string;
		detail: string;
		options: AgentHold[];
		value: AgentHold;
		disabled: boolean;
		onchange: (hold: AgentHold) => void;
	} = $props();
</script>

<div class="flex flex-col gap-1.5">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<span class="text-sm leading-normal text-ink-900">{title}</span>
		<Select.Root
			type="single"
			{value}
			{disabled}
			onValueChange={(chosen) => onchange(chosen as AgentHold)}
		>
			<Select.Trigger class="w-48" aria-label="{title}: when to hold">
				{holdOptionLabels[value] ?? "Never hold"}
			</Select.Trigger>
			<Select.Content>
				{#each options as option (option)}
					<Select.Item value={option} label={holdOptionLabels[option]}>
						{holdOptionLabels[option]}
					</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<span class="text-xs leading-normal text-muted-foreground text-pretty">{detail}</span>
	<span class="text-xs leading-normal text-muted-foreground text-pretty">
		{holdOptionHints[value] ?? holdOptionHints.never}
	</span>
</div>
