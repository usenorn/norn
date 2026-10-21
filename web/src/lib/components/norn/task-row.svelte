<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PersonAvatar from "./person-avatar.svelte";
	import PriorityIcon from "./priority-icon.svelte";
	import StatusIcon from "./status-icon.svelte";
	import LabelChips from "$lib/labels/label-chips.svelte";
	import { cn } from "$lib/utils.js";
	import type { RowProperty } from "$lib/issues/display";
	import type { Task } from "$lib/tasks/types";

	let {
		task,
		href,
		cursor = false,
		selected = false,
		draggable = false,
		dragging = false,
		ondragstart,
		ondragend,
		shown: showing = ["labels", "due"],
		class: className,
	}: {
		task: Task;
		href: string;
		cursor?: boolean;
		selected?: boolean;
		draggable?: boolean;
		dragging?: boolean;
		ondragstart?: (event: DragEvent) => void;
		ondragend?: (event: DragEvent) => void;
		shown?: readonly RowProperty[];
		class?: string;
	} = $props();

	const labels = $derived(showing.includes("labels") ? task.labels : []);
	const settled = $derived(
		task.state.category === "complete" || task.state.category === "abandoned"
	);
	const line = $derived(
		[
			task.id,
			task.date && showing.includes("due") ? task.date : "",
			task.assignee?.split(" ")[0] ?? "",
		]
			.filter(Boolean)
			.join(" · ")
	);
</script>

<a
	{href}
	{draggable}
	{ondragstart}
	{ondragend}
	data-cursor={cursor}
	aria-current={cursor ? "true" : undefined}
	data-selected={selected}
	data-dragging={dragging}
	class={cn(
		"flex min-h-12 items-center gap-3 border-b border-line-subtle bg-card px-4 py-1.5 motion-row cursor-row hover:bg-accent data-[selected=true]:rule-lead data-[selected=true]:bg-surface-selected data-[dragging=true]:opacity-40 sm:h-row sm:min-h-0 sm:gap-2 sm:px-row-x sm:py-0",
		draggable && "cursor-grab active:cursor-grabbing",
		className
	)}
>
	<PriorityIcon priority={task.priority} class="size-icon-row order-last sm:order-none" />
	<span class="order-first flex flex-none sm:order-none">
		<StatusIcon category={task.state.category} name={task.state.name} />
	</span>
	<span
		class="hidden w-[var(--reference-width,7ch)] flex-none font-mono text-xs text-muted-foreground data-[strong=true]:text-ink-900 sm:block"
		data-strong={cursor || selected}
	>
		{task.id}
	</span>
	<span class="flex min-w-0 flex-1 flex-col gap-0.5 sm:block">
		<span
			class="block truncate text-base font-medium tracking-snug sm:text-md sm:font-normal {settled
				? 'text-muted-foreground'
				: 'text-ink-900'}"
		>
			{task.title}
		</span>
		<span class="truncate tabular-nums text-2xs text-muted-foreground sm:hidden">{line}</span>
	</span>
	<span class="hidden flex-none items-center justify-end gap-2.5 text-xs text-muted-foreground sm:flex">
		<LabelChips {labels} place="row" />
		{#if task.date && showing.includes("due")}
			<span class="hidden tabular-nums text-xs whitespace-nowrap text-muted-foreground sm:inline">
				{task.date}
			</span>
		{/if}
		{#if task.assignee}
			<PersonAvatar
				accountId={task.assigneeAccountId ?? ""}
				name={task.assignee}
				size="xs"
				title={task.assignee}
			/>
		{:else}
			<Avatar.Root size="xs" variant="ghost" title="Unassigned">
				<Avatar.Fallback>+</Avatar.Fallback>
			</Avatar.Root>
		{/if}
	</span>
</a>
