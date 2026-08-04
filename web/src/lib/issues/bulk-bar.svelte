<script lang="ts">
	import Archive from "@lucide/svelte/icons/archive";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import X from "@lucide/svelte/icons/x";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { priorities, type IssuePriority } from "$lib/issues/issues";
	import type { WorkflowState } from "$lib/team/states";
	import type { Member } from "$lib/issues/members";

	let {
		count,
		states,
		members,
		working = false,
		onpriority,
		onstate,
		onassignee,
		onstatus,
		onclear,
	}: {
		count: number;
		states: WorkflowState[];
		members: Member[];
		working?: boolean;
		onpriority: (priority: IssuePriority) => void;
		onstate: (stateId: string) => void;
		onassignee: (accountId: string) => void;
		onstatus: (status: "archived" | "pending_deletion") => void;
		onclear: () => void;
	} = $props();
</script>

<div
	class="sticky bottom-0 z-10 flex flex-wrap items-center gap-2 border-t border-line-default bg-card px-4 py-2"
	role="region"
	aria-label="Bulk actions"
>
	<span class="font-mono text-xs text-ink-900 tabular-nums">
		{count} selected
	</span>

	<span class="h-4 w-px bg-line-default" aria-hidden="true"></span>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline" size="sm" disabled={working}>
					State
					<ChevronDown aria-hidden="true" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
			<DropdownMenu.Group>
				<DropdownMenu.GroupHeading>Move all to</DropdownMenu.GroupHeading>
				{#each states as state (state.id)}
					<DropdownMenu.Item onSelect={() => onstate(state.id)}>
						<StatusIcon category={state.category} decorative />
						{state.name}
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Group>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline" size="sm" disabled={working}>
					Priority
					<ChevronDown aria-hidden="true" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			<DropdownMenu.Group>
				<DropdownMenu.GroupHeading>Set all to</DropdownMenu.GroupHeading>
				{#each priorities as choice (choice.value)}
					<DropdownMenu.Item onSelect={() => onpriority(choice.value)}>
						<PriorityIcon priority={choice.value} />
						{choice.label}
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Group>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline" size="sm" disabled={working}>
					Assignee
					<ChevronDown aria-hidden="true" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
			<DropdownMenu.Group>
				<DropdownMenu.GroupHeading>Assign all to</DropdownMenu.GroupHeading>
				<DropdownMenu.Item onSelect={() => onassignee("")}>Unassigned</DropdownMenu.Item>
				<DropdownMenu.Separator />
				{#each members as member (member.accountId)}
					<DropdownMenu.Item onSelect={() => onassignee(member.accountId)}>
						{member.displayName || member.email}
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Group>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<span class="h-4 w-px bg-line-default" aria-hidden="true"></span>

	<Button variant="outline" size="sm" disabled={working} onclick={() => onstatus("archived")}>
		<Archive aria-hidden="true" />
		Archive
	</Button>

	<Button variant="ghost" size="sm" disabled={working} onclick={() => onstatus("pending_deletion")}>
		<Trash2 aria-hidden="true" />
		Delete
	</Button>

	<span class="flex-1"></span>

	<Button variant="ghost" size="sm" aria-label="Clear selection" onclick={onclear}>
		<X aria-hidden="true" />
	</Button>
</div>
