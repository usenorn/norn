<script lang="ts">
	import type { Snippet } from "svelte";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { cn } from "$lib/utils.js";
	import { dueLabel, overdue } from "$lib/time";
	import type { RowProperty } from "./display";
	import type { Issue } from "./issues";

	let {
		issue,
		href,
		assignee = "",
		now = "",
		timezone = "UTC",
		selected = false,
		onselect,
		shown = ["labels", "due"],
		priorityControl,
		stateControl,
		labelsControl,
		assigneeControl,
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
		selected?: boolean;
		onselect?: (extend: boolean) => void;
		shown?: RowProperty[];
		priorityControl?: Snippet<[Issue]>;
		stateControl?: Snippet<[Issue]>;
		labelsControl?: Snippet<[Issue, boolean]>;
		assigneeControl?: Snippet<[Issue]>;
		draggable?: boolean;
		dragging?: boolean;
		ondragstart?: (event: DragEvent) => void;
		ondragend?: (event: DragEvent) => void;
		class?: string;
	} = $props();

	const labels = $derived(shown.includes("labels") ? issue.labels : []);
	const due = $derived(shown.includes("due") ? issue.dueOn : undefined);
	const visible = $derived(labels.slice(0, 2));
	const hidden = $derived(labels.length - visible.length);
	const settled = $derived(
		issue.state.category === "complete" || issue.state.category === "abandoned"
	);
	const late = $derived(!settled && overdue(due, now));

	const initials = $derived(
		assignee
			.trim()
			.split(/\s+/)
			.slice(0, 2)
			.map((part) => part[0])
			.join("")
	);
</script>

<div
	role="listitem"
	data-issue={issue.id}
	data-selected={selected}
	data-dragging={dragging}
	{draggable}
	{ondragstart}
	{ondragend}
	class={cn(
		"relative flex flex-col gap-2 rounded-lg border border-line-default bg-card px-3 py-2.5 motion-control hover:border-ink-400 active:bg-paper-2 data-[selected=true]:rule-lead data-[selected=true]:border-ink-400 data-[dragging=true]:opacity-40",
		draggable && "cursor-grab active:cursor-grabbing",
		className
	)}
>
	<div class="flex items-center gap-1.5">
		{#if stateControl}
			<span class="relative z-1 flex">{@render stateControl(issue)}</span>
		{:else}
			<StatusIcon category={issue.state.category} name={issue.state.name} />
		{/if}
		<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
		<span class="flex-1"></span>
		{#if priorityControl}
			<span class="relative z-1 flex">{@render priorityControl(issue)}</span>
		{:else}
			<PriorityIcon priority={issue.priority} class="size-icon-row" />
		{/if}
	</div>

	<a
		{href}
		draggable="false"
		onclick={(event) => {
			if (!onselect || !(event.metaKey || event.ctrlKey || event.shiftKey)) return;

			event.preventDefault();
			onselect(event.shiftKey);
		}}
		class="text-md leading-snug font-medium tracking-snug text-ink-900 after:absolute after:inset-0"
	>
		{issue.title}
	</a>

	<div class="flex min-h-5 items-center gap-1.5">
		<span class="relative z-1 flex min-w-0 items-center gap-1.5">
			{#if labelsControl}
				{@render labelsControl(issue, true)}
			{:else}
				{#each visible as label (label.id)}
					<Tag name={label.name} color={label.color} />
				{/each}
				{#if hidden > 0}
					<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">+{hidden}</span>
				{/if}
			{/if}
		</span>
		<span class="min-w-1.5 flex-1"></span>
		{#if due}
			<span
				class="font-mono text-2xs whitespace-nowrap {late
					? 'text-priority-urgent'
					: 'text-muted-foreground'}"
			>
				{dueLabel(due, now, timezone)}
			</span>
		{/if}
		<span class="relative z-1 flex flex-none">
			{#if assigneeControl}
				{@render assigneeControl(issue)}
			{:else if assignee}
				<Avatar.Root size="xs" title={assignee}>
					<Avatar.Fallback>{initials}</Avatar.Fallback>
				</Avatar.Root>
			{:else}
				<Avatar.Root size="xs" variant="ghost" title="Unassigned">
					<Avatar.Fallback>+</Avatar.Fallback>
				</Avatar.Root>
			{/if}
		</span>
	</div>
</div>
