<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PriorityIcon from "./priority-icon.svelte";
	import StatusIcon from "./status-icon.svelte";
	import Tag from "./tag.svelte";
	import { cn } from "$lib/utils.js";
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
		class?: string;
	} = $props();

	const shown = $derived(task.labels.slice(0, 2));
	const hidden = $derived(task.labels.length - shown.length);
	const done = $derived(task.status === "done" || task.status === "canceled");

	const initials = $derived(
		(task.assignee ?? "")
			.trim()
			.split(/\s+/)
			.slice(0, 2)
			.map((part) => part[0])
			.join("")
	);
</script>

<a
	{href}
	{draggable}
	{ondragstart}
	{ondragend}
	data-cursor={cursor}
	data-selected={selected}
	data-dragging={dragging}
	class={cn(
		"flex h-row items-center gap-2 border-b border-line-subtle bg-card px-row-x transition-none hover:bg-accent hover:transition-colors hover:duration-70 hover:ease-out data-[cursor=true]:rule-lead data-[cursor=true]:bg-surface-cursor data-[selected=true]:rule-lead data-[selected=true]:bg-surface-selected data-[dragging=true]:opacity-40",
		draggable && "cursor-grab active:cursor-grabbing",
		className
	)}
>
	<PriorityIcon priority={task.priority} class="size-icon-row" />
	<StatusIcon status={task.status} />
	<span
		class="w-15 flex-none font-mono text-xs text-muted-foreground data-[strong=true]:text-ink-900"
		data-strong={cursor || selected}
	>
		{task.id}
	</span>
	<span
		class="min-w-0 flex-1 truncate text-md tracking-snug {done
			? 'text-muted-foreground'
			: 'text-ink-900'}"
	>
		{task.title}
	</span>
	<span class="flex flex-none items-center justify-end gap-2.5 text-xs text-muted-foreground">
		{#each shown as label (label.name)}
			<Tag name={label.name} color={label.color} class="hidden lg:inline-flex" />
		{/each}
		{#if hidden > 0}
			<span class="hidden font-mono text-2xs text-muted-foreground lg:inline">+{hidden}</span>
		{/if}
		{#if task.date}
			<span class="hidden font-mono text-xs whitespace-nowrap text-muted-foreground sm:inline">
				{task.date}
			</span>
		{/if}
		{#if task.assignee}
			<Avatar.Root size="xs" title={task.assignee}>
				<Avatar.Fallback>{initials}</Avatar.Fallback>
			</Avatar.Root>
		{:else}
			<Avatar.Root size="xs" variant="ghost" title="Unassigned">
				<Avatar.Fallback>+</Avatar.Fallback>
			</Avatar.Root>
		{/if}
	</span>
</a>
