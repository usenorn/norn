<script lang="ts">
	import type { Snippet } from "svelte";
	import Check from "@lucide/svelte/icons/check";
	import Link2 from "@lucide/svelte/icons/link-2";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PriorityIcon from "./priority-icon.svelte";
	import StatusIcon from "./status-icon.svelte";
	import ProgressBar from "./progress-bar.svelte";
	import Tag from "./tag.svelte";
	import { cn } from "$lib/utils.js";
	import { dueLabel, overdue } from "$lib/time";
	import { totalIssues } from "$lib/issues/board";
	import type { RowProperty } from "$lib/issues/display";
	import type { Issue } from "$lib/issues/issues";

	let {
		issue,
		href,
		assignee = "",
		now = "",
		timezone = "UTC",
		cursor = false,
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
		cursor?: boolean;
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
	const children = $derived(issue.childProgress);
	const hasChildren = $derived(children ? totalIssues(children) > 0 : false);

	const initials = $derived(
		assignee
			.trim()
			.split(/\s+/)
			.slice(0, 2)
			.map((part) => part[0])
			.join("")
	);

	const line = $derived(
		[issue.reference, due ? dueLabel(due, now, timezone) : "", assignee.split(" ")[0] ?? ""]
			.filter(Boolean)
			.join(" · ")
	);
</script>

<div
	data-cursor={cursor}
	data-selected={selected}
	data-dragging={dragging}
	class={cn(
		"group/row relative flex min-h-12 items-center gap-3 border-b border-line-subtle bg-card px-4 py-1.5 transition-none sm:h-row sm:min-h-0 sm:gap-2 sm:px-row-x sm:py-0 hover:bg-accent hover:transition-colors hover:duration-70 hover:ease-out data-[cursor=true]:rule-lead data-[cursor=true]:bg-surface-cursor data-[selected=true]:rule-lead data-[selected=true]:bg-surface-selected data-[dragging=true]:opacity-40",
		draggable && "cursor-grab active:cursor-grabbing",
		className
	)}
>
	{#if onselect}
		<button
			type="button"
			role="checkbox"
			aria-checked={selected}
			aria-label="Select {issue.reference}"
			onclick={(event) => onselect(event.shiftKey)}
			data-on={selected || cursor}
			class="relative z-1 -mr-0.5 hidden h-row w-4 flex-none cursor-pointer items-center justify-center opacity-0 sm:flex transition-opacity duration-70 ease-out group-hover/row:opacity-100 focus-visible:opacity-100 data-[on=true]:opacity-100"
		>
			<span
				data-on={selected}
				class="flex size-3.25 items-center justify-center rounded-xs border border-line-strong text-primary-foreground transition-colors duration-110 ease-out data-[on=true]:border-primary data-[on=true]:bg-primary"
			>
				{#if selected}
					<Check class="size-2.5" aria-hidden="true" />
				{/if}
			</span>
		</button>
	{/if}

	{#if priorityControl}
		<span class="relative z-1 order-last flex flex-none sm:order-none">
			{@render priorityControl(issue)}
		</span>
	{:else}
		<PriorityIcon priority={issue.priority} class="size-icon-row order-last sm:order-none" />
	{/if}

	<span
		class="hidden w-15 flex-none font-mono text-xs text-muted-foreground data-[strong=true]:text-ink-900 sm:block"
		data-strong={cursor || selected}
	>
		{issue.reference}
	</span>

	{#if stateControl}
		<span class="relative z-1 order-first flex flex-none sm:order-none">
			{@render stateControl(issue)}
		</span>
	{:else}
		<span class="order-first flex flex-none sm:order-none">
			<StatusIcon category={issue.state.category} name={issue.state.name} />
		</span>
	{/if}

	<span class="flex min-w-0 flex-1 flex-col gap-0.5 sm:block">
		<a
			{href}
			{draggable}
			{ondragstart}
			{ondragend}
			onclick={(event) => {
				if (!onselect || !(event.metaKey || event.ctrlKey || event.shiftKey)) return;

				event.preventDefault();
				onselect(event.shiftKey);
			}}
			class="block truncate text-base font-medium tracking-snug after:absolute after:inset-0 sm:text-md sm:font-normal {settled
				? 'text-muted-foreground'
				: 'text-ink-900'}"
		>
			{issue.title}
		</a>
		<span class="truncate font-mono text-2xs text-muted-foreground sm:hidden">{line}</span>
	</span>

	<span
		class="relative z-1 hidden flex-none items-center justify-end gap-2.5 text-xs text-muted-foreground sm:flex"
	>
		{#if issue.blocked}
			<Link2 class="size-icon-row shrink-0 text-priority-urgent" aria-label="Blocked" />
		{/if}
		{#if hasChildren && children}
			<ProgressBar progress={children} label={false} class="hidden md:inline-flex" />
		{/if}
		{#if labelsControl}
			{@render labelsControl(issue, false)}
		{:else}
			{#each visible as label (label.id)}
				<Tag name={label.name} color={label.color} class="hidden lg:inline-flex" />
			{/each}
			{#if hidden > 0}
				<span class="hidden font-mono text-2xs text-muted-foreground lg:inline">+{hidden}</span>
			{/if}
		{/if}
		{#if issue.estimate}
			<span class="hidden font-mono text-2xs text-muted-foreground sm:inline">
				{issue.estimate}
			</span>
		{/if}
		{#if due}
			<span
				class="hidden font-mono text-xs whitespace-nowrap sm:inline {late
					? 'text-priority-urgent'
					: 'text-muted-foreground'}"
			>
				{dueLabel(due, now, timezone)}
			</span>
		{/if}
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
