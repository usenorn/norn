<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";
	import { fieldTrigger } from "#lib/issues/issue-field.svelte";

	export const labelPickerVariants = tv({
		slots: {
			trigger: "",
			blank: "",
			content: "",
		},
		variants: {
			place: {
				field: { trigger: fieldTrigger({ class: "items-start" }), blank: "text-muted-foreground" },
				row: {
					trigger:
						"relative inline-flex h-6 min-w-5 cursor-pointer items-center gap-1.5 rounded-sm px-1 hover:bg-paper-2 pointer-coarse:after:absolute pointer-coarse:after:-inset-2.5",
					blank: "size-3.5 text-muted-foreground",
					content: "w-49",
				},
				dialog: {
					trigger: "",
					content: "w-49",
				},
			},
			always: {
				true: {},
				false: {},
			},
		},
		compoundVariants: [
			{
				place: "row",
				always: false,
				class: {
					blank:
						"hidden opacity-0 group-hover/row:opacity-100 group-focus-within/row:opacity-100 lg:inline-block",
				},
			},
			{
				place: "row",
				always: true,
				class: {
					blank:
						"opacity-0 group-hover/card:opacity-100 group-focus-within/card:opacity-100",
				},
			},
		],
		defaultVariants: { place: "field", always: false },
	});

	export type LabelPickerPlace = NonNullable<VariantProps<typeof labelPickerVariants>["place"]>;
</script>

<script lang="ts">
	import Plus from "@lucide/svelte/icons/plus";
	import Tags from "@lucide/svelte/icons/tags";
	import * as Command from "#lib/components/ui/command/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import PropertyPicker, { type PickerOption } from "#lib/issues/property-picker.svelte";
	import LabelChips from "./label-chips.svelte";
	import LabelDot from "./label-dot.svelte";
	import { createLabel } from "./create-label";
	import {
		labelFailureMessage,
		selectable,
		toggled,
		type Label,
		type LabelFailure,
	} from "./labels";

	let {
		workspaceId,
		labels,
		chosen,
		teamId,
		place = "field",
		always = false,
		chips = true,
		canCreate = false,
		disabled = false,
		subject,
		open = $bindable(false),
		onchange,
		oncreated,
	}: {
		workspaceId: string;
		labels: Label[];
		chosen: Label[];
		teamId?: string;
		place?: LabelPickerPlace;
		always?: boolean;
		chips?: boolean;
		canCreate?: boolean;
		disabled?: boolean;
		subject?: string;
		open?: boolean;
		onchange: (labelIds: string[], picked: Label) => void;
		oncreated?: (label: Label) => void;
	} = $props();

	let search = $state("");
	let creating = $state(false);
	let failure = $state<LabelFailure | null>(null);

	const styles = $derived(labelPickerVariants({ place, always }));
	const shownChips = $derived(chips ? chosen : []);
	const reachable = $derived(teamId === undefined ? labels : selectable(labels, teamId));
	const options = $derived<PickerOption[]>(
		reachable.map((label) => ({
			value: label.id,
			label: label.name,
			checked: chosen.some((held) => held.id === label.id),
		}))
	);
	const wanted = $derived(search.trim());
	const creatable = $derived(
		wanted.length > 0 &&
			!reachable.some((label) => label.name.toLowerCase() === wanted.toLowerCase())
	);
	const triggerLabel = $derived(
		place === "row" && subject
			? `Change labels on ${subject}`
			: `Labels: ${shownChips.map((label) => label.name).join(", ") || "none"}`
	);

	$effect(() => {
		if (!open) failure = null;
	});

	function pick(labelId: string) {
		const label = reachable.find((candidate) => candidate.id === labelId);

		if (!label) return;

		failure = null;
		onchange(toggled(chosen, label), label);
	}

	async function create() {
		if (!creatable || creating) return;

		creating = true;
		failure = null;

		const created = await createLabel(workspaceId, wanted);

		creating = false;

		if (created.kind === "failed") {
			failure = created.failure;

			return;
		}

		search = "";
		oncreated?.(created.label);
		onchange(toggled(chosen, created.label), created.label);
	}
</script>

{#snippet createRow(typed: string)}
	{#if creatable}
		<Command.Item value="create-{typed}" disabled={creating} onSelect={create}>
			<span class="inline-flex w-3.75 flex-none justify-center">
				<Plus class="text-muted-foreground" aria-hidden="true" />
			</span>
			<span class="min-w-0 flex-1 truncate">
				{creating ? "Creating" : "Create"} “{wanted}”
			</span>
		</Command.Item>
	{:else if options.length === 0}
		<p class="px-2 py-1.5 text-sm text-muted-foreground">
			No labels reach this team. Type a name to create one.
		</p>
	{/if}
{/snippet}

<PropertyPicker
	{options}
	placeholder={canCreate ? "Add or create a label…" : "Add label…"}
	class={styles.content()}
	align={place === "row" ? "end" : "start"}
	empty="No labels reach this team"
	closeOnPick={false}
	bind:open
	bind:search
	onpick={pick}
	action={canCreate ? createRow : undefined}
>
	{#snippet trigger(props)}
		{#if place === "dialog"}
			<Button {...props} variant="property" size="property" {disabled}>
				<Tags class="text-muted-foreground" aria-hidden="true" />
				{#if shownChips.length > 0}
					<LabelChips labels={shownChips} place="card" />
				{:else}
					Label
				{/if}
			</Button>
		{:else}
			<button
				{...props}
				type="button"
				aria-label={triggerLabel}
				data-property={place === "field" ? "Labels" : undefined}
				{disabled}
				class={styles.trigger()}
			>
				{#if place === "field" && shownChips.length === 0}
					<Tags class="size-icon-row text-muted-foreground" aria-hidden="true" />
				{/if}
				{#if shownChips.length > 0}
					<LabelChips labels={shownChips} place={place === "row" && always ? "card" : place} />
				{/if}
				{#if shownChips.length === 0}
					{#if place === "field"}
						<span class={styles.blank()}>Add labels</span>
					{:else}
						<Tags class={styles.blank()} aria-hidden="true" />
					{/if}
				{/if}
			</button>
		{/if}
	{/snippet}
	{#snippet mark(option)}
		<LabelDot color={reachable.find((label) => label.id === option.value)?.color} />
	{/snippet}
	{#snippet footer()}
		{#if failure}
			<p class="border-t border-line-subtle px-2 py-1.5 text-sm text-destructive" role="alert">
				{labelFailureMessage(failure)}
			</p>
		{/if}
	{/snippet}
</PropertyPicker>
