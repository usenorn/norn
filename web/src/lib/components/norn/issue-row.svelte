<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PriorityIcon from "./priority-icon.svelte";
	import StatusIcon from "./status-icon.svelte";
	import Tag from "./tag.svelte";
	import { cn } from "$lib/utils.js";
	import { dueLabel, overdue } from "$lib/time";
	import type { Issue } from "$lib/issues/issues";

	let {
		issue,
		href,
		assignee = "",
		now = "",
		timezone = "UTC",
		cursor = false,
		selected = false,
		draggable = false,
		dragging = false,
		ondragstart,
		ondragend,
		class: className,
	}: {
		issue: Issue;
		href: string;
		assignee?: string;
		now?: string;
		timezone?: string;
		cursor?: boolean;
		selected?: boolean;
		draggable?: boolean;
		dragging?: boolean;
		ondragstart?: (event: DragEvent) => void;
		ondragend?: (event: DragEvent) => void;
		class?: string;
	} = $props();

	const shown = $derived(issue.labels.slice(0, 2));
	const hidden = $derived(issue.labels.length - shown.length);
	const settled = $derived(
		issue.state.category === "complete" || issue.state.category === "abandoned"
	);
	const late = $derived(!settled && overdue(issue.dueOn, now));

	const initials = $derived(
		assignee
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
	<PriorityIcon priority={issue.priority} class="size-icon-row" />
	<StatusIcon category={issue.state.category} name={issue.state.name} />
	<span
		class="w-15 flex-none font-mono text-xs text-muted-foreground data-[strong=true]:text-ink-900"
		data-strong={cursor || selected}
	>
		{issue.reference}
	</span>
	<span
		class="min-w-0 flex-1 truncate text-md tracking-snug {settled
			? 'text-muted-foreground'
			: 'text-ink-900'}"
	>
		{issue.title}
	</span>
	<span class="flex flex-none items-center justify-end gap-2.5 text-xs text-muted-foreground">
		{#each shown as label (label.id)}
			<Tag name={label.name} color={label.color} class="hidden lg:inline-flex" />
		{/each}
		{#if hidden > 0}
			<span class="hidden font-mono text-2xs text-muted-foreground lg:inline">+{hidden}</span>
		{/if}
		{#if issue.estimate}
			<span class="hidden font-mono text-2xs text-muted-foreground sm:inline">
				{issue.estimate}
			</span>
		{/if}
		{#if issue.dueOn}
			<span
				class="hidden font-mono text-xs whitespace-nowrap sm:inline {late
					? 'text-priority-urgent'
					: 'text-muted-foreground'}"
			>
				{dueLabel(issue.dueOn, now, timezone)}
			</span>
		{/if}
		{#if assignee}
			<Avatar.Root size="xs" title={assignee}>
				<Avatar.Fallback>{initials}</Avatar.Fallback>
			</Avatar.Root>
		{:else}
			<Avatar.Root size="xs" variant="ghost" title="Unassigned">
				<Avatar.Fallback>+</Avatar.Fallback>
			</Avatar.Root>
		{/if}
	</span>
</a>
