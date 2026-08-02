<script lang="ts">
	import Bell from "@lucide/svelte/icons/bell";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import Funnel from "@lucide/svelte/icons/funnel";
	import Kanban from "@lucide/svelte/icons/kanban";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Plus from "@lucide/svelte/icons/plus";
	import X from "@lucide/svelte/icons/x";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import TaskRow from "$lib/components/norn/task-row.svelte";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		countForTab,
		filterLabels,
		groupIssues,
		groupLabels,
		groupKeys,
		order,
		orderKeys,
		orderLabels,
		selectIssues,
		type GroupKey,
		type IssueFilters,
		type OrderKey,
	} from "$lib/tasks/grouping";
	import { workspacePath } from "$lib/workspace/navigation";
	import type { Task } from "$lib/tasks/types";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const at = $derived((path: string) => workspacePath(slug, path));

	const params = $derived.by(() => {
		const q = new URLSearchParams();
		if (data.tab !== "active") q.set("tab", data.tab);
		if (data.layout !== "list") q.set("layout", data.layout);
		if (data.group !== "status") q.set("group", data.group);
		if (data.order !== "manual") q.set("order", data.order);
		if (data.showEmpty) q.set("empty", "1");
		for (const [key, value] of Object.entries(data.filters)) q.set(key, value);
		return q;
	});

	const linkWith = $derived((changes: Record<string, string | null>) => {
		const q = new URLSearchParams(params);
		for (const [key, value] of Object.entries(changes)) {
			if (value === null) q.delete(key);
			else q.set(key, value);
		}
		const query = q.toString();
		return at(`/issues${query ? `?${query}` : ""}`);
	});

	let overrides = $state<Record<string, Partial<Task>>>({});
	let dragging = $state<string | null>(null);
	let dropTarget = $state<string | null>(null);

	const tasks = $derived(
		data.issues.map((task) => (overrides[task.id] ? { ...task, ...overrides[task.id] } : task))
	);

	const dimension = $derived<keyof Task | null>(
		data.group === "none" ? null : (data.group as keyof Task)
	);

	function onDragStart(event: DragEvent, id: string) {
		if (!dimension) return;
		dragging = id;
		event.dataTransfer?.setData("text/plain", id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
	}

	function onDragEnd() {
		dragging = null;
		dropTarget = null;
	}

	function onDragOver(event: DragEvent, key: string) {
		if (!dimension || !dragging) return;
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
		dropTarget = key;
	}

	function onDrop(event: DragEvent, group: { key: string }) {
		event.preventDefault();
		const id = event.dataTransfer?.getData("text/plain") || dragging;
		onDragEnd();
		if (!id || !dimension) return;
		const value = data.group === "assignee" && group.key === "unassigned" ? null : group.key;
		const patch: Partial<Task> = { [dimension]: value } as Partial<Task>;
		if (data.group === "status") patch.cycle = group.key === "backlog" ? null : "24";
		overrides = { ...overrides, [id]: { ...overrides[id], ...patch } };
	}

	const issues = $derived(selectIssues(tasks, data.tab, data.filters));
	const ordered = $derived(order(issues, data.order));
	const groups = $derived(
		groupIssues(ordered, data.group, {
			layout: data.layout,
			showEmpty: data.showEmpty,
			people: ["Rae Okafor", "Jun Park", "Milo Vance", "Ada Ling"],
			projects: data.projects,
		})
	);

	const tabs = $derived([
		{ value: "active" as const, label: "Active" },
		{ value: "backlog" as const, label: "Backlog" },
		{ value: "all" as const, label: "All issues" },
	]);

	const cycleIssues = $derived(tasks.filter((task) => task.cycle === "24"));
	const cycleDone = $derived(cycleIssues.filter((task) => task.status === "done").length);
	const donePercent = $derived(
		cycleIssues.length ? Math.round((cycleDone / cycleIssues.length) * 100) : 0
	);

	const activeFilters = $derived(
		Object.entries(data.filters) as [keyof IssueFilters, string][]
	);

	const flat = $derived(groups.flatMap((group) => group.tasks));
	let cursor = $state(0);

	function onkeydown(event: KeyboardEvent) {
		if (event.metaKey || event.ctrlKey || event.altKey) return;
		const target = event.target as HTMLElement | null;
		if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)))
			return;
		if (event.key === "j" || event.key === "ArrowDown") {
			event.preventDefault();
			cursor = Math.min(cursor + 1, flat.length - 1);
		} else if (event.key === "k" || event.key === "ArrowUp") {
			event.preventDefault();
			cursor = Math.max(cursor - 1, 0);
		}
	}
</script>

<svelte:head><title>Issues · Norn</title></svelte:head>
<svelte:window {onkeydown} />

<div class="relative flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<List class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Issues</h1>
				<div role="tablist" class="ml-1 hidden shrink-0 items-center gap-3 sm:flex">
					{#each tabs as tab (tab.value)}
						{@const count = countForTab(tasks, tab.value, data.filters)}
						<a
							href={linkWith({ tab: tab.value === "active" ? null : tab.value })}
							role="tab"
							aria-selected={data.tab === tab.value}
							data-active={data.tab === tab.value}
							class="relative inline-flex h-7.5 items-center gap-1.5 text-md font-medium whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-transparent after:transition-colors after:duration-110 after:ease-out hover:text-ink-900 hover:after:bg-line-strong data-[active=true]:text-ink-900 data-[active=true]:after:bg-primary"
						>
							{tab.label}
							<span class="font-mono text-2xs text-muted-foreground tabular-nums">{count}</span>
						</a>
					{/each}
				</div>
				{#if data.filters.cycle}
					<span class="hidden shrink-0 items-center gap-2 whitespace-nowrap lg:inline-flex">
						<span class="font-mono text-xs text-muted-foreground">Jul 29 – Aug 12</span>
						<span class="inline-block h-[3px] w-16 overflow-hidden rounded-xs bg-paper-3">
							<span class="block h-[3px] rounded-xs bg-primary" style="width: {donePercent}%"
							></span>
						</span>
						<span class="font-mono text-xs text-muted-foreground tabular-nums">
							{cycleDone}/{cycleIssues.length} done
						</span>
					</span>
				{/if}
			</div>
			<div class="flex flex-none gap-1">
				<Button
					href={linkWith({ layout: null })}
					variant="outline"
					size="icon-sm"
					aria-label="List view"
					aria-pressed={data.layout === "list"}
					class={data.layout === "list" ? "border-primary bg-primary text-primary-foreground" : ""}
				>
					<List class="size-icon-toolbar" aria-hidden="true" />
				</Button>
				<Button
					href={linkWith({ layout: "board" })}
					variant="outline"
					size="icon-sm"
					aria-label="Board view"
					aria-pressed={data.layout === "board"}
					class={data.layout === "board" ? "border-primary bg-primary text-primary-foreground" : ""}
				>
					<Kanban class="size-icon-toolbar" aria-hidden="true" />
				</Button>
			</div>
			<Button variant="outline" size="icon-sm" aria-label="Notifications">
				<Bell class="size-icon-toolbar" aria-hidden="true" />
			</Button>
			<Button size="sm">
				<Plus aria-hidden="true" />
				New issue
			</Button>
		</div>

		<div class="flex h-8.5 items-center gap-3 overflow-hidden border-t border-line-subtle pr-3 pl-3.5">
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="sm">
							<Funnel aria-hidden="true" />
							Filter
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="start">
					<DropdownMenu.Label>Filter by</DropdownMenu.Label>
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={linkWith({ priority: "urgent" })} {...props}>
								<PriorityIcon priority="urgent" class="size-icon-row" />
								Urgent priority
							</a>
						{/snippet}
					</DropdownMenu.Item>
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={linkWith({ status: "started" })} {...props}>
								<StatusIcon status="started" />
								In progress
							</a>
						{/snippet}
					</DropdownMenu.Item>
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={linkWith({ cycle: "24" })} {...props}>
								<Layers aria-hidden="true" />
								Cycle 24
							</a>
						{/snippet}
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					{#each data.projects as project (project.slug)}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a href={linkWith({ project: project.slug })} {...props}>
									<span
										class="size-2 shrink-0 rounded-xs"
										style="background: {project.color}"
										aria-hidden="true"
									></span>
									{project.name}
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/each}
				</DropdownMenu.Content>
			</DropdownMenu.Root>

			{#each activeFilters as [key, value] (key)}
				<span
					class="inline-flex items-center gap-1.5 border-b-2 border-line-strong pb-0.5 text-sm whitespace-nowrap text-foreground"
				>
					{filterLabels[key]}: {value}
					<a href={linkWith({ [key]: null })} aria-label="Remove {filterLabels[key]} filter">
						<X class="size-3 text-muted-foreground hover:text-ink-900" aria-hidden="true" />
					</a>
				</span>
			{/each}
			{#if activeFilters.length > 1}
				<a
					href={at("/issues")}
					class="text-sm text-muted-foreground hover:text-foreground"
				>
					Clear all
				</a>
			{/if}

			<div class="min-w-2 flex-1"></div>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="sm" class="whitespace-nowrap">
							Grouped by {groupLabels[data.group]}
							<ChevronDown aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.Label>Group by</DropdownMenu.Label>
					{#each groupKeys as key (key)}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a href={linkWith({ group: key === "status" ? null : key })} {...props}>
									<span class="flex-1 capitalize">{groupLabels[key]}</span>
									{#if data.group === key}
										<span class="font-mono text-2xs text-ink-600">✓</span>
									{/if}
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/each}
					<DropdownMenu.Separator />
					<DropdownMenu.Label>Order by</DropdownMenu.Label>
					{#each orderKeys as key (key)}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a href={linkWith({ order: key === "manual" ? null : key })} {...props}>
									<span class="flex-1">{orderLabels[key as OrderKey]}</span>
									{#if data.order === key}
										<span class="font-mono text-2xs text-ink-600">✓</span>
									{/if}
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/each}
					<DropdownMenu.Separator />
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={linkWith({ empty: data.showEmpty ? null : "1" })} {...props}>
								<span class="flex-1">Show empty groups</span>
								{#if data.showEmpty}
									<span class="font-mono text-2xs text-ink-600">✓</span>
								{/if}
							</a>
						{/snippet}
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</div>

	<div class="relative flex min-h-0 flex-1 flex-col">
		{#if issues.length === 0}
			<div class="flex flex-1 flex-col items-center justify-center gap-3">
				<span class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">
					No issues match these filters
				</span>
				<Button href={at("/issues")} variant="secondary" size="sm">Clear filters</Button>
			</div>
		{:else if data.layout === "list"}
			<div class="flex-1 overflow-auto">
				{#each groups as group (group.key)}
					{@const offset = flat.indexOf(group.tasks[0])}
					<section
						role="group"
						aria-label={group.label}
						ondragover={(event) => onDragOver(event, group.key)}
						ondrop={(event) => onDrop(event, group)}
						data-dropping={dropTarget === group.key}
						class="data-[dropping=true]:bg-accent"
					>
						<div
							class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-1.5"
						>
							<span class="flex h-6 items-center gap-1.5 rounded-sm px-1">
								{#if group.status}
									<StatusIcon status={group.status} />
								{:else if group.priority}
									<PriorityIcon priority={group.priority} class="size-icon-row" />
								{:else if group.dot}
									<span
										class="size-2 rounded-xs"
										style="background: {group.dot}"
										aria-hidden="true"
									></span>
								{/if}
								<span
									class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
								>
									{group.label}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{group.tasks.length}
								</span>
							</span>
							<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
							<Button variant="ghost" size="icon-xs" aria-label="New issue in {group.label}">
								<Plus aria-hidden="true" />
							</Button>
						</div>
						{#each group.tasks as task, index (task.id)}
							<TaskRow
								{task}
								href={at(`/issues/${task.id}`)}
								cursor={offset + index === cursor}
								draggable={dimension !== null}
								dragging={dragging === task.id}
								ondragstart={(event) => onDragStart(event, task.id)}
								ondragend={onDragEnd}
							/>
						{/each}
					</section>
				{/each}
			</div>
		{:else}
			<div class="flex-1 overflow-auto bg-background p-4">
				<div class="flex min-h-full items-start gap-3">
					{#each groups as group (group.key)}
						<div
							role="group"
							aria-label={group.label}
							ondragover={(event) => onDragOver(event, group.key)}
							ondrop={(event) => onDrop(event, group)}
							data-dropping={dropTarget === group.key}
							class="flex w-62.5 flex-none flex-col gap-2 rounded-lg border border-transparent p-1 transition-colors duration-70 ease-out data-[dropping=true]:border-dashed data-[dropping=true]:border-ink-400 data-[dropping=true]:bg-accent"
						>
							<div class="flex h-7 items-center gap-2 px-1">
								{#if group.status}
									<StatusIcon status={group.status} />
								{:else if group.priority}
									<PriorityIcon priority={group.priority} class="size-icon-row" />
								{:else if group.dot}
									<span
										class="size-2 rounded-xs"
										style="background: {group.dot}"
										aria-hidden="true"
									></span>
								{/if}
								<span
									class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
								>
									{group.label}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{group.tasks.length}
								</span>
								<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
								<Button variant="ghost" size="icon-xs" aria-label="New issue in {group.label}">
									<Plus aria-hidden="true" />
								</Button>
							</div>
							{#each group.tasks as task (task.id)}
								<a
									href={at(`/issues/${task.id}`)}
									draggable={dimension !== null}
									ondragstart={(event) => onDragStart(event, task.id)}
									ondragend={onDragEnd}
									data-dragging={dragging === task.id}
									class="flex cursor-grab flex-col gap-2 rounded-lg border border-line-default bg-card p-2.5 transition-colors duration-70 ease-out hover:border-ink-400 active:cursor-grabbing data-[dragging=true]:opacity-40"
								>
									<div class="flex items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground">{task.id}</span>
										<span class="flex-1"></span>
										<PriorityIcon priority={task.priority} class="size-icon-row" />
									</div>
									<span class="text-md leading-snug tracking-snug text-ink-900">{task.title}</span>
									<div class="flex items-center gap-2">
										{#each task.labels.slice(0, 2) as label (label.name)}
											<Tag name={label.name} color={label.color} />
										{/each}
										<span class="flex-1"></span>
										{#if task.date}
											<span class="font-mono text-xs text-muted-foreground">{task.date}</span>
										{/if}
										{#if task.assignee}
											<Avatar.Root size="xs" title={task.assignee}>
												<Avatar.Fallback>
													{task.assignee
														.split(/\s+/)
														.slice(0, 2)
														.map((part) => part[0])
														.join("")}
												</Avatar.Fallback>
											</Avatar.Root>
										{/if}
									</div>
								</a>
							{/each}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<div
		class="hidden h-7.5 flex-none items-center justify-end gap-4 border-t border-line-subtle bg-card px-3.5 md:flex"
	>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="C" />new issue
		</span>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="⌘ K" />go anywhere
		</span>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="↑ ↓" />move
		</span>
	</div>
</div>
