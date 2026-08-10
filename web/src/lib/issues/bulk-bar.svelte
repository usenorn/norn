<script lang="ts" module>
	export type BulkPicker = "state" | "assignee" | "cycle" | "more";
</script>

<script lang="ts">
	import Archive from "@lucide/svelte/icons/archive";
	import Ellipsis from "@lucide/svelte/icons/ellipsis";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import X from "@lucide/svelte/icons/x";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { initialsOf } from "$lib/team/members";
	import PropertyPicker, { type PickerOption } from "./property-picker.svelte";
	import { priorities, type IssuePriority } from "./issues";
	import type { Cycle } from "$lib/cycles/cycles";
	import type { WorkflowState } from "$lib/team/states";
	import type { Member } from "./members";

	let {
		count,
		states,
		members,
		cycles,
		working = false,
		onpriority,
		onstate,
		onassignee,
		oncycle,
		onstatus,
		onclear,
	}: {
		count: number;
		states: WorkflowState[];
		members: Member[];
		cycles: Cycle[];
		working?: boolean;
		onpriority: (priority: IssuePriority) => void;
		onstate: (stateId: string) => void;
		onassignee: (accountId: string) => void;
		oncycle: (cycleId: string) => void;
		onstatus: (status: "archived" | "pending_deletion") => void;
		onclear: () => void;
	} = $props();

	let pickingState = $state(false);
	let pickingAssignee = $state(false);
	let pickingCycle = $state(false);
	let pickingMore = $state(false);

	export function pick(what: BulkPicker) {
		pickingState = what === "state";
		pickingAssignee = what === "assignee";
		pickingCycle = what === "cycle";
		pickingMore = what === "more";
	}

	const stateOptions = $derived<PickerOption[]>(
		states.map((state) => ({ value: state.id, label: state.name }))
	);

	const assigneeOptions = $derived<PickerOption[]>([
		...members.map((member) => ({
			value: member.accountId,
			label: member.displayName || member.email || "Someone",
		})),
		{ value: "", label: "Unassigned" },
	]);

	const cycleOptions = $derived<PickerOption[]>(
		cycles.map((cycle) => ({ value: cycle.id, label: cycle.name }))
	);

	const barButton =
		"h-control-sm border-line-inverse bg-transparent px-2 text-sm text-primary-foreground shadow-none hover:border-line-inverse hover:bg-paper-0/9 active:bg-paper-0/14";
</script>

<div
	role="region"
	aria-label="Bulk actions"
	class="flex min-h-8.5 flex-none flex-wrap items-center gap-2.5 bg-primary py-1 pr-2.5 pl-3.5"
>
	<span class="font-mono text-xs whitespace-nowrap text-primary-foreground tabular-nums">
		{count} selected
	</span>

	<span class="h-4 w-px bg-line-inverse" aria-hidden="true"></span>

	<PropertyPicker
		bind:open={pickingState}
		options={stateOptions}
		placeholder="Set status…"
		onpick={onstate}
	>
		{#snippet trigger(props)}
			<Button
				{...props}
				variant="outline"
				size="sm"
				disabled={working || stateOptions.length === 0}
				class={barButton}
			>
				Set status
			</Button>
		{/snippet}
		{#snippet mark(option)}
			{@const state = states.find((candidate) => candidate.id === option.value)}
			{#if state}
				<StatusIcon category={state.category} decorative />
			{/if}
		{/snippet}
	</PropertyPicker>

	<PropertyPicker
		bind:open={pickingAssignee}
		options={assigneeOptions}
		placeholder="Assign to…"
		onpick={onassignee}
	>
		{#snippet trigger(props)}
			<Button {...props} variant="outline" size="sm" disabled={working} class={barButton}>
				Assign
			</Button>
		{/snippet}
		{#snippet mark(option)}
			{#if option.value}
				<Avatar.Root size="xs">
					<Avatar.Fallback>{initialsOf(option.label)}</Avatar.Fallback>
				</Avatar.Root>
			{:else}
				<Avatar.Root size="xs" variant="ghost">
					<Avatar.Fallback>+</Avatar.Fallback>
				</Avatar.Root>
			{/if}
		{/snippet}
	</PropertyPicker>

	<PropertyPicker
		bind:open={pickingCycle}
		options={cycleOptions}
		placeholder="Move to cycle…"
		empty="No cycle is open on this team"
		onpick={oncycle}
	>
		{#snippet trigger(props)}
			<Button {...props} variant="outline" size="sm" disabled={working} class={barButton}>
				Move to cycle
			</Button>
		{/snippet}
	</PropertyPicker>

	<DropdownMenu.Root bind:open={pickingMore}>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="outline"
					size="sm"
					disabled={working}
					aria-label="More bulk actions"
					class={barButton}
				>
					<Ellipsis aria-hidden="true" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			<DropdownMenu.Group>
				<DropdownMenu.GroupHeading>Set priority</DropdownMenu.GroupHeading>
				{#each priorities as choice (choice.value)}
					<DropdownMenu.Item onSelect={() => onpriority(choice.value)}>
						<PriorityIcon priority={choice.value} />
						{choice.label}
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Group>
			<DropdownMenu.Separator />
			<DropdownMenu.Group>
				<DropdownMenu.Item onSelect={() => onstatus("archived")}>
					<Archive aria-hidden="true" />
					Archive
				</DropdownMenu.Item>
				<DropdownMenu.Item variant="destructive" onSelect={() => onstatus("pending_deletion")}>
					<Trash2 aria-hidden="true" />
					Delete
				</DropdownMenu.Item>
			</DropdownMenu.Group>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<span class="flex-1"></span>

	<span
		class="hidden items-center gap-1.5 text-xs whitespace-nowrap text-paper-0/74 sm:inline-flex"
	>
		<Kbd
			keys="Esc"
			class="border-line-inverse bg-transparent text-paper-0/74 [--keycap-lip:rgba(15,17,22,0.14)]"
		/>
		clear
	</span>

	<Button
		variant="ghost"
		size="icon-sm"
		aria-label="Clear selection"
		onclick={onclear}
		class="text-primary-foreground hover:bg-paper-0/9 hover:text-primary-foreground"
	>
		<X aria-hidden="true" />
	</Button>
</div>
