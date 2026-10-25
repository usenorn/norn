<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";
	import { fieldTrigger } from "#lib/issues/issue-field.svelte";

	export const priorityPickerVariants = tv({
		slots: {
			trigger: "",
			content: "w-49",
		},
		variants: {
			place: {
				field: { trigger: fieldTrigger() },
				row: {
					trigger:
						"relative inline-flex h-6 w-5 cursor-pointer items-center justify-center rounded-sm hover:bg-paper-2 pointer-coarse:after:absolute pointer-coarse:after:-inset-2.5",
				},
				dialog: {
					trigger: "",
				},
			},
		},
		defaultVariants: { place: "field" },
	});

	export type PriorityPickerPlace = NonNullable<
		VariantProps<typeof priorityPickerVariants>["place"]
	>;
</script>

<script lang="ts">
	import { Button } from "#lib/components/ui/button/index.js";
	import PriorityIcon from "#lib/components/norn/priority-icon.svelte";
	import PropertyPicker, { type PickerOption } from "#lib/issues/property-picker.svelte";
	import { priorities, priorityLabel, type IssuePriority } from "./issues";

	let {
		chosen,
		place = "field",
		disabled = false,
		subject,
		open = $bindable(false),
		onchange,
	}: {
		chosen: IssuePriority;
		place?: PriorityPickerPlace;
		disabled?: boolean;
		subject?: string;
		open?: boolean;
		onchange: (priority: IssuePriority) => void;
	} = $props();

	const styles = $derived(priorityPickerVariants({ place }));
	const options = $derived<PickerOption[]>(
		priorities.map((entry) => ({
			value: entry.value,
			label: entry.label,
			checked: entry.value === chosen,
		}))
	);
	const triggerLabel = $derived(
		place === "row" && subject ? `Change priority on ${subject}` : `Priority: ${priorityLabel(chosen)}`
	);
</script>

<PropertyPicker
	{options}
	placeholder="Set priority…"
	class={styles.content()}
	bind:open
	onpick={(value) => onchange(value as IssuePriority)}
>
	{#snippet trigger(props)}
		{#if place === "dialog"}
			<Button {...props} variant="property" size="property" {disabled}>
				<PriorityIcon priority={chosen} />
				{chosen === "none" ? "Priority" : priorityLabel(chosen)}
			</Button>
		{:else}
			<button
				{...props}
				type="button"
				aria-label={triggerLabel}
				data-property={place === "field" ? "Priority" : undefined}
				{disabled}
				class={styles.trigger()}
			>
				<PriorityIcon priority={chosen} class={place === "row" ? "size-icon-row" : undefined} />
				{#if place === "field"}
					<span class="min-w-0 flex-1 truncate">{priorityLabel(chosen)}</span>
				{/if}
			</button>
		{/if}
	{/snippet}
	{#snippet mark(option)}
		<PriorityIcon priority={option.value as IssuePriority} />
	{/snippet}
</PropertyPicker>
