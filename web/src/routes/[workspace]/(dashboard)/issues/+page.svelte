<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Bell from "@lucide/svelte/icons/bell";
	import CalendarDays from "@lucide/svelte/icons/calendar-days";
	import Check from "@lucide/svelte/icons/check";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import CircleHelp from "@lucide/svelte/icons/circle-help";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Folder from "@lucide/svelte/icons/folder";
	import Funnel from "@lucide/svelte/icons/funnel";
	import Kanban from "@lucide/svelte/icons/kanban";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Plus from "@lucide/svelte/icons/plus";
	import Settings from "@lucide/svelte/icons/settings";
	import Tags from "@lucide/svelte/icons/tags";
	import Users from "@lucide/svelte/icons/users";
	import X from "@lucide/svelte/icons/x";
	import { SvelteSet } from "svelte/reactivity";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import Toast from "$lib/components/norn/toast.svelte";
	import LabelDot from "$lib/labels/label-dot.svelte";
	import BulkBar from "$lib/issues/bulk-bar.svelte";
	import BulkResult from "$lib/issues/bulk-result.svelte";
	import IssueCard from "$lib/issues/issue-card.svelte";
	import NewIssueDialog from "$lib/issues/new-issue-dialog.svelte";
	import PropertyPicker, { type PickerOption } from "$lib/issues/property-picker.svelte";
	import { rangeBetween, settled, type BulkActionResult } from "$lib/issues/bulk";
	import { api } from "$lib/api";
	import {
		backlogStates,
		boardFor,
		tabCounts,
		type ColumnSource,
		type Issue,
		type IssueColumn,
	} from "$lib/issues/board";
	import ColumnMore from "$lib/issues/column-more.svelte";
	import {
		atDefaults,
		groupingLabels,
		groupingNouns,
		groupings,
		hiddenParam,
		issueTabs,
		orderingLabels,
		orderings,
		rowProperties,
		rowPropertyLabels,
		tabLabels,
		writeDisplay,
	} from "$lib/issues/display";
	import {
		noPages,
		pagesOf,
		withFailure,
		withLoading,
		withPage,
		type BoardPages,
	} from "$lib/issues/paging";
	import { columnQuery, tallyTotal } from "$lib/issues/filter";
	import {
		columnFilter,
		dueWindowLabels,
		dueWindows,
		facetCount,
		facetLabels,
		pickableFacets,
		unassigned,
		type FacetKind,
	} from "$lib/issues/facets";
	import { issueFailureMessage, priorities, priorityLabel, readIssueFailure } from "$lib/issues/issues";
	import type { IssuePriority } from "$lib/issues/issues";
	import { brokenIn } from "$lib/views/applied";
	import { referenceLabel, scopeOf, viewsPath } from "$lib/views/views";
	import { initialsOf } from "$lib/team/members";
	import type { WorkflowState } from "$lib/team/states";
	import { workspacePath } from "$lib/workspace/navigation";
	import { cycleWindow } from "$lib/time";
	import { issuesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? issuesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	const slug = $derived(data.workspace.slug);
	const at = $derived((path: string) => workspacePath(slug, path));

	let pages = $state.raw<BoardPages>(noPages);

	let dragging = $state<string | null>(null);
	let dropTarget = $state<string | null>(null);
	let failure = $state<string | null>(null);
	let toast = $state<{ message: string; undo?: () => Promise<void> } | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;

	const team = $derived(preview?.team ?? data.team);
	const states = $derived(preview?.states ?? data.states ?? []);
	const progress = $derived(preview?.progress ?? data.progress);
	const members = $derived(preview?.members ?? data.members ?? []);
	const labels = $derived(preview?.labels ?? data.labels ?? []);
	const display = $derived(preview?.display ?? data.display);
	const facets = $derived(preview?.facets ?? data.facets);

	const base = $derived(preview?.issues ?? data.issues);
	const tallies = $derived(preview?.groups ?? data.groups);

	const source = $derived<ColumnSource | undefined>(
		base && {
			issues: base,
			tallies,
			nextCursor: preview?.nextCursor ?? data.nextCursor,
		}
	);

	const held = $derived(preview?.pages ?? pagesOf(pages, base));

	const applied = $derived(data.applied);
	const broken = $derived(brokenIn(applied));
	const scopeName = $derived(applied.kind === "applied" ? applied.view.name : (team?.name ?? ""));

	const teamCycles = $derived(
		(data.cycles ?? []).filter((entry) => !team || entry.teamId === team.id).map((entry) => entry.cycle)
	);
	const openCycle = $derived(teamCycles.find((cycle) => cycle.id === facets.cycle));

	const board = $derived(
		boardFor(
			source,
			display.grouping,
			{ states, members, projects: data.projects ?? [] },
			held,
			{ name: scopeName, teams: data.teams.length },
			{ showEmpty: display.showEmpty }
		)
	);
	const columns = $derived(board.kind === "ready" ? board.columns : []);
	const names = $derived(
		new Map(members.map((member) => [member.accountId, member.displayName ?? ""]))
	);
	const flat = $derived(columns.flatMap((column) => column.issues));
	const offsets = $derived.by(() => {
		const at = new Map<string, number>();
		let seen = 0;

		for (const column of columns) {
			at.set(column.key, seen);
			seen += column.issues.length;
		}

		return at;
	});
	const statesByTeam = $derived.by(() => {
		const byTeam: Record<string, WorkflowState[]> = {};

		for (const state of states) byTeam[state.teamId] = [...(byTeam[state.teamId] ?? []), state];

		return byTeam;
	});
	const statesOfTeam = $derived((teamId: string) => statesByTeam[teamId] ?? []);
	const backlog = $derived(backlogStates(states));
	const counts = $derived(tabCounts(preview?.totals ?? data.totals, states, backlog));
	const total = $derived(tallyTotal(tallies) ?? flat.length);

	const params = $derived.by(() => {
		const q = new URLSearchParams();

		if (applied.kind === "applied") q.set("view", applied.view.id);
		else if (applied.kind === "gone") q.set("view", applied.id);
		else if (team) q.set("team", team.key);

		for (const [kind, value] of Object.entries(facets)) if (value) q.set(kind, value);

		for (const [key, value] of writeDisplay(display, data.layout, data.tab)) q.set(key, value);

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

	const cleared = $derived(
		linkWith(Object.fromEntries(pickableFacets.map((kind) => [kind, null])) as Record<string, null>)
	);

	$effect(() => () => clearTimeout(toastTimer));

	function announce(message: string, undo?: () => Promise<void>) {
		clearTimeout(toastTimer);
		toast = { message, undo };
		toastTimer = setTimeout(() => (toast = null), 6000);
	}

	async function patch(issue: Issue, body: Record<string, unknown>): Promise<boolean> {
		failure = null;

		const { error } = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
			params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
			body: { expectedVersion: issue.version, ...body },
		});

		if (error) {
			failure = issueFailureMessage(readIssueFailure(error));
			await invalidate(keys.page(page.route.id));

			return false;
		}

		await invalidate(keys.page(page.route.id));

		return true;
	}

	function asLoaded(issue: Issue): Issue {
		return flat.find((candidate) => candidate.id === issue.id) ?? issue;
	}

	async function change(
		issue: Issue,
		body: Record<string, unknown>,
		previous: Record<string, unknown>,
		message: string
	) {
		if (!(await patch(issue, body))) return;

		announce(message, async () => {
			toast = null;
			await patch(asLoaded(issue), previous);
		});
	}

	async function setState(issue: Issue, stateId: string) {
		if (issue.state.id === stateId) return;

		const name = states.find((state) => state.id === stateId)?.name ?? "another status";

		await change(
			issue,
			{ stateId },
			{ stateId: issue.state.id },
			`Moved ${issue.reference} to ${name}`
		);
	}

	async function setPriority(issue: Issue, priority: IssuePriority) {
		if (issue.priority === priority) return;

		await change(
			issue,
			{ priority },
			{ priority: issue.priority },
			`Set ${issue.reference} to ${priorityLabel(priority).toLowerCase()}`
		);
	}

	async function setAssignee(issue: Issue, accountId: string) {
		if ((issue.assigneeAccountId ?? "") === accountId) return;

		const held = issue.assigneeAccountId;
		const name = names.get(accountId) ?? "";

		await change(
			issue,
			accountId === "" ? { clear: ["assignee"] } : { assigneeId: accountId },
			held ? { assigneeId: held } : { clear: ["assignee"] },
			accountId === ""
				? `Unassigned ${issue.reference}`
				: `Assigned ${issue.reference} to ${name}`
		);
	}

	async function toggleLabel(issue: Issue, labelId: string) {
		const held = issue.labels.map((label) => label.id);
		const carries = held.includes(labelId);
		const next = carries ? held.filter((id) => id !== labelId) : [...held, labelId];
		const name = labels.find((label) => label.id === labelId)?.name ?? "that label";

		failure = null;

		const { error } = await api.PUT("/workspaces/{workspaceId}/issues/{issueId}/labels", {
			params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
			body: { expectedVersion: issue.version, labelIds: next },
		});

		if (error) {
			failure = issueFailureMessage(readIssueFailure(error));

			return;
		}

		await invalidate(keys.page(page.route.id));

		announce(
			carries
				? `Removed ${name} from ${issue.reference}`
				: `Added ${name} to ${issue.reference}`,
			async () => {
				toast = null;

				const fresh = asLoaded(issue);

				await api.PUT("/workspaces/{workspaceId}/issues/{issueId}/labels", {
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: fresh.version, labelIds: held },
				});

				await invalidate(keys.page(page.route.id));
			}
		);
	}

	async function loadColumn(column: IssueColumn) {
		if (column.load.kind === "complete" || column.load.kind === "loading") return;

		const from = base;

		pages = withLoading(pages, from, column.key);

		try {
			const { data: next, error } = await api.POST("/workspaces/{workspaceId}/issues/query", {
				params: { path: { workspaceId: data.workspace.id } },
				body: columnQuery(
					data.query,
					columnFilter(display.grouping, column.key),
					column.load.cursor
				),
			});

			if (error || !next) {
				pages = withFailure(pages, from, column.key);

				return;
			}

			pages = withPage(pages, from, column.key, next.issues, next.nextCursor);
		} catch {
			pages = withFailure(pages, from, column.key);
		}
	}

	function onDragStart(event: DragEvent, id: string) {
		dragging = id;
		event.dataTransfer?.setData("text/plain", id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
	}

	function onDragEnd() {
		dragging = null;
		dropTarget = null;
	}

	function landsIn(id: string | null, key: string): boolean {
		const issue = flat.find((candidate) => candidate.id === id);
		const target = states.find((state) => state.id === key);

		return Boolean(issue) && (!target || target.teamId === issue?.teamId);
	}

	function onDragOver(event: DragEvent, key: string) {
		if (!dragging || display.grouping !== "state" || !landsIn(dragging, key)) return;

		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
		dropTarget = key;
	}

	function onDrop(event: DragEvent, key: string) {
		event.preventDefault();

		const id = event.dataTransfer?.getData("text/plain") || dragging;
		onDragEnd();

		if (display.grouping !== "state" || !landsIn(id, key)) return;

		const issue = flat.find((candidate) => candidate.id === id);
		if (issue) void setState(issue, key);
	}

	let saving = $state(false);
	let viewName = $state("");
	let savingView = $state(false);

	async function saveView(event: SubmitEvent) {
		event.preventDefault();

		if (!viewName.trim()) return;

		savingView = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/saved-views", {
				params: { path: { workspaceId: data.workspace.id } },
				body: {
					name: viewName.trim(),
					sharing: "personal",
					filter: data.query.filter,
					sort: data.query.sort,
					groupBy: data.query.groupBy,
				},
			});

			if (error) {
				failure = "That view was not saved. Nothing changed — try again.";

				return;
			}

			viewName = "";
			saving = false;
			await invalidate(keys.page(page.route.id));
		} catch {
			failure = "That view was not saved. Nothing changed — try again.";
		} finally {
			savingView = false;
		}
	}

	let cursor = $state(0);
	let selected = $state(new SvelteSet<string>());
	let anchor = $state<string | null>(null);
	let liveBulk = $state<BulkActionResult | null>(null);
	let applying = $state(false);
	let polling = $state<ReturnType<typeof setTimeout> | null>(null);
	let collapsed = $state(new SvelteSet<string>());
	let creating = $state(false);
	let prefill = $state<Record<string, string> | undefined>(undefined);
	let filterOpen = $state(false);
	let filterCategory = $state<FacetKind | null>(null);
	let filterSearch = $state("");
	let displayOpen = $state(false);
	let displayPane = $state<"root" | "grouping" | "ordering">("root");

	$effect(() => {
		if (!displayOpen) displayPane = "root";
	});

	const bulk = $derived(preview?.bulk ?? liveBulk);
	const orderedIDs = $derived(flat.map((issue) => issue.id));

	const selectedTeams = $derived(
		new Set(flat.filter((issue) => selected.has(issue.id)).map((issue) => issue.teamId))
	);
	const sharedStates = $derived(
		selectedTeams.size === 1 ? statesOfTeam([...selectedTeams][0]) : []
	);

	function toggle(id: string, extend = false) {
		if (extend && anchor) {
			for (const between of rangeBetween(orderedIDs, anchor, id)) selected.add(between);
		} else if (selected.has(id)) {
			selected.delete(id);
			anchor = id;
		} else {
			selected.add(id);
			anchor = id;
		}
	}

	function clearSelection() {
		selected.clear();
		anchor = null;
		liveBulk = null;
	}

	async function poll(actionId: string) {
		const { data: latest } = await api.GET(
			"/workspaces/{workspaceId}/bulk-actions/{bulkActionId}",
			{ params: { path: { workspaceId: data.workspace.id, bulkActionId: actionId } } }
		);

		if (!latest) return;

		liveBulk = latest;

		if (settled(latest.status)) {
			await invalidate(keys.page(page.route.id));

			return;
		}

		polling = setTimeout(() => poll(actionId), 700);
	}

	async function applyBulk(change: Record<string, unknown>) {
		if (selected.size === 0) return;

		applying = true;
		liveBulk = null;

		if (polling) clearTimeout(polling);

		try {
			const { data: result, error } = await api.POST("/workspaces/{workspaceId}/issues/bulk", {
				params: { path: { workspaceId: data.workspace.id } },
				body: { change, issueIds: [...selected] },
			});

			if (error || !result) {
				failure = issueFailureMessage(readIssueFailure(error));

				return;
			}

			liveBulk = result;

			if (settled(result.status)) {
				selected.clear();
				anchor = null;
				await invalidate(keys.page(page.route.id));
			} else {
				polling = setTimeout(() => poll(result.id), 700);
			}
		} catch {
			failure = "Something went wrong and nothing changed. Wait a moment and try again.";
		} finally {
			applying = false;
		}
	}

	function raise(seed?: Record<string, string>) {
		prefill = { teamId: team?.id ?? "", ...seed };
		creating = true;
	}

	function seedFor(key: string): Record<string, string> {
		switch (display.grouping) {
			case "state":
				return { stateId: key };
			case "priority":
				return { priority: key };
			case "assignee":
				return key ? { assigneeId: key } : {};
			case "project":
				return key ? { projectId: key } : {};
			default:
				return {};
		}
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.metaKey || event.ctrlKey || event.altKey) return;

		const target = event.target as HTMLElement | null;

		if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)))
			return;

		if (creating) return;

		if (event.key === "x" || event.key === " ") {
			const issue = flat[cursor];

			if (issue) {
				event.preventDefault();
				toggle(issue.id);
			}

			return;
		}

		if (event.key === "c" || event.key === "C") {
			event.preventDefault();
			raise();

			return;
		}

		if (event.key === "f" || event.key === "F") {
			event.preventDefault();
			filterCategory = null;
			filterOpen = true;

			return;
		}

		if (event.key === "1") {
			void goto(linkWith({ layout: null }));

			return;
		}

		if (event.key === "2") {
			void goto(linkWith({ layout: "board" }));

			return;
		}

		if (event.key === "Escape" && selected.size > 0) {
			event.preventDefault();
			clearSelection();

			return;
		}

		if (event.key === "j" || event.key === "ArrowDown") {
			event.preventDefault();
			cursor = Math.min(cursor + 1, flat.length - 1);
		} else if (event.key === "k" || event.key === "ArrowUp") {
			event.preventDefault();
			cursor = Math.max(cursor - 1, 0);
		}
	}

	const facetOptions = $derived.by((): PickerOption[] => {
		const valuesFor = (kind: FacetKind): PickerOption[] => {
			switch (kind) {
				case "state":
					return states.map((state) => ({
						value: state.id,
						label: state.name,
						checked: facets.state === state.id,
					}));
				case "assignee":
					return [
						...members.map((member) => ({
							value: member.accountId,
							label: member.displayName ?? "Someone",
							checked: facets.assignee === member.accountId,
						})),
						{ value: unassigned, label: "Unassigned", checked: facets.assignee === unassigned },
					];
				case "priority":
					return priorities.map((entry) => ({
						value: entry.value,
						label: entry.label,
						checked: facets.priority === entry.value,
					}));
				case "label":
					return labels.map((label) => ({
						value: label.id,
						label: label.name,
						checked: facets.label === label.id,
					}));
				case "project":
					return (data.projects ?? []).map((project) => ({
						value: project.id,
						label: project.name,
						checked: facets.project === project.id,
					}));
				default:
					return dueWindows.map((window) => ({
						value: window,
						label: dueWindowLabels[window],
						checked: facets.due === window,
					}));
			}
		};

		if (filterCategory) {
			return [
				{ value: "", label: "All properties" },
				...valuesFor(filterCategory).map((option) => ({
					...option,
					href: linkWith({ [filterCategory as string]: option.checked ? null : option.value }),
				})),
			];
		}

		if (filterSearch.trim() === "") {
			return pickableFacets.map((kind) => ({
				value: kind,
				label: facetLabels[kind],
				trailing: true,
			}));
		}

		return pickableFacets.flatMap((kind) =>
			valuesFor(kind).map((option) => ({
				...option,
				value: `${kind}:${option.value}`,
				label: `${facetLabels[kind]} · ${option.label}`,
				href: linkWith({ [kind]: option.checked ? null : option.value }),
			}))
		);
	});

	function pickFacet(value: string) {
		if (value === "") {
			filterCategory = null;

			return;
		}

		if (!filterCategory && pickableFacets.includes(value as FacetKind)) {
			filterCategory = value as FacetKind;
			filterOpen = true;
		}
	}

	const chips = $derived.by(() => {
		const named = (kind: FacetKind, value: string): string => {
			switch (kind) {
				case "state":
					return states.find((state) => state.id === value)?.name ?? "Unknown status";
				case "assignee":
					return value === unassigned ? "Unassigned" : (names.get(value) ?? "Unknown person");
				case "priority":
					return priorityLabel(value as IssuePriority);
				case "label":
					return labels.find((label) => label.id === value)?.name ?? "Unknown label";
				case "project":
					return (data.projects ?? []).find((project) => project.id === value)?.name ?? "Unknown project";
				case "cycle":
					return teamCycles.find((cycle) => cycle.id === value)?.name ?? "Unknown cycle";
				default:
					return dueWindowLabels[value as keyof typeof dueWindowLabels] ?? value;
			}
		};

		return Object.entries(facets)
			.filter(([, value]) => Boolean(value))
			.map(([kind, value]) => ({
				kind: kind as FacetKind,
				label: `${facetLabels[kind as FacetKind]}: ${named(kind as FacetKind, value)}`,
			}));
	});

	const filtered = $derived(facetCount(facets) > 0);
</script>

<svelte:head><title>Issues · Norn</title></svelte:head>
<svelte:window {onkeydown} />

{#snippet columnMark(column: (typeof columns)[number])}
	{#if column.mark.kind === "state"}
		<StatusIcon category={column.mark.state.category} name={column.mark.state.name} />
	{:else if column.mark.kind === "priority"}
		<PriorityIcon priority={column.mark.priority} />
	{:else if column.mark.kind === "assignee"}
		{#if column.mark.name}
			<Avatar.Root size="xs" title={column.mark.name}>
				<Avatar.Fallback>{initialsOf(column.mark.name)}</Avatar.Fallback>
			</Avatar.Root>
		{:else}
			<Avatar.Root size="xs" variant="ghost" title="Unassigned">
				<Avatar.Fallback>+</Avatar.Fallback>
			</Avatar.Root>
		{/if}
	{:else if column.mark.kind === "project"}
		<Folder class="size-icon-row text-muted-foreground" aria-hidden="true" />
	{:else if column.mark.kind === "unknown"}
		<CircleHelp class="size-icon-row text-muted-foreground" aria-hidden="true" />
	{:else}
		<Layers class="size-icon-row text-muted-foreground" aria-hidden="true" />
	{/if}
{/snippet}

{#snippet priorityControl(issue: Issue)}
	<PropertyPicker
		options={priorities.map((entry) => ({
			value: entry.value,
			label: entry.label,
			checked: entry.value === issue.priority,
		}))}
		placeholder="Set priority…"
		class="w-49"
		onpick={(value) => setPriority(issue, value as IssuePriority)}
	>
		{#snippet trigger(props)}
			<button
				{...props}
				type="button"
				aria-label="Change priority on {issue.reference}"
				class="inline-flex h-6 w-5 cursor-pointer items-center justify-center rounded-sm hover:bg-paper-2"
			>
				<PriorityIcon priority={issue.priority} class="size-icon-row" />
			</button>
		{/snippet}
		{#snippet mark(option)}
			<PriorityIcon priority={option.value as IssuePriority} />
		{/snippet}
	</PropertyPicker>
{/snippet}

{#snippet stateControl(issue: Issue)}
	<PropertyPicker
		options={statesOfTeam(issue.teamId).map((state) => ({
			value: state.id,
			label: state.name,
			checked: state.id === issue.state.id,
		}))}
		placeholder="Set status…"
		onpick={(value) => setState(issue, value)}
	>
		{#snippet trigger(props)}
			<button
				{...props}
				type="button"
				aria-label="Change status on {issue.reference}"
				class="inline-flex h-6 w-5.5 cursor-pointer items-center justify-center rounded-sm hover:bg-paper-2"
			>
				<StatusIcon category={issue.state.category} name={issue.state.name} />
			</button>
		{/snippet}
		{#snippet mark(option)}
			{@const state = states.find((candidate) => candidate.id === option.value)}
			{#if state}
				<StatusIcon category={state.category} decorative />
			{/if}
		{/snippet}
	</PropertyPicker>
{/snippet}

{#snippet labelsControl(issue: Issue, always = false)}
	{@const carried = display.shown.includes("labels") ? issue.labels : []}
	<PropertyPicker
		options={labels.map((label) => ({
			value: label.id,
			label: label.name,
			checked: carried.some((held) => held.id === label.id),
		}))}
		placeholder="Add label…"
		class="w-49"
		align="end"
		closeOnPick={false}
		empty="No labels reach this team"
		onpick={(value) => toggleLabel(issue, value)}
	>
		{#snippet shortcut()}
			<Kbd keys="Esc" />
		{/snippet}
		{#snippet trigger(props)}
			<button
				{...props}
				type="button"
				aria-label="Change labels on {issue.reference}"
				class="inline-flex h-6 min-w-5 cursor-pointer items-center gap-1.5 rounded-sm px-1 hover:bg-paper-2"
			>
				{#each carried.slice(0, 2) as label (label.id)}
					<Tag name={label.name} color={label.color} class="hidden lg:inline-flex" />
				{/each}
				{#if carried.length > 2}
					<span class="hidden font-mono text-2xs text-muted-foreground lg:inline">
						+{carried.length - 2}
					</span>
				{/if}
				{#if carried.length === 0}
					<span
						class="h-0.5 w-2 bg-line-strong {always
							? ''
							: 'hidden opacity-0 group-hover/row:opacity-100 lg:inline-block'}"
						aria-hidden="true"
					></span>
				{/if}
			</button>
		{/snippet}
		{#snippet mark(option)}
			{@const label = labels.find((candidate) => candidate.id === option.value)}
			<LabelDot color={label?.color} />
		{/snippet}
	</PropertyPicker>
{/snippet}

{#snippet assigneeControl(issue: Issue)}
	{@const held = names.get(issue.assigneeAccountId ?? "") ?? ""}
	<PropertyPicker
		options={[
			...members.map((member) => ({
				value: member.accountId,
				label: member.displayName ?? "Someone",
				checked: member.accountId === issue.assigneeAccountId,
			})),
			{ value: "", label: "Unassigned", checked: !issue.assigneeAccountId },
		]}
		placeholder="Assign to…"
		align="end"
		onpick={(value) => setAssignee(issue, value)}
	>
		{#snippet trigger(props)}
			<button
				{...props}
				type="button"
				aria-label="Change assignee on {issue.reference}"
				class="inline-flex size-6 cursor-pointer items-center justify-center rounded-sm hover:bg-paper-2"
			>
				{#if held}
					<Avatar.Root size="xs" title={held}>
						<Avatar.Fallback>{initialsOf(held)}</Avatar.Fallback>
					</Avatar.Root>
				{:else}
					<Avatar.Root size="xs" variant="ghost" title="Unassigned">
						<Avatar.Fallback>+</Avatar.Fallback>
					</Avatar.Root>
				{/if}
			</button>
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
{/snippet}

<div class="relative flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex min-h-11 flex-wrap items-center gap-2 py-1.5 pr-3 pl-4 sm:flex-nowrap sm:py-0">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<List class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Issues</h1>

				{#if applied.kind === "applied"}
					<span class="flex min-w-0 shrink items-center gap-1.5">
						<span class="truncate text-md text-ink-900">{applied.view.name}</span>
						<span class="shrink-0 font-mono text-2xs tracking-eyebrow text-ink-600 uppercase">
							{scopeOf(applied.view)}
						</span>
					</span>
				{:else if data.teams.length > 1 && team}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="ghost" size="sm" class="shrink-0">
									<TeamKey key={team.key} />
									{team.name}
									<ChevronDown aria-hidden="true" />
								</Button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="start">
							<DropdownMenu.Label>Team</DropdownMenu.Label>
							{#each data.teams as candidate (candidate.id)}
								<DropdownMenu.Item>
									{#snippet child({ props })}
										<a href={linkWith({ team: candidate.key })} {...props}>
											<span class="flex-1">{candidate.name}</span>
											{#if candidate.id === team.id}
												<span class="font-mono text-2xs text-ink-600">✓</span>
											{/if}
										</a>
									{/snippet}
								</DropdownMenu.Item>
							{/each}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{:else if team}
					<span class="shrink-0 text-md text-muted-foreground">{team.name}</span>
				{/if}

				{#if source}
					<div
						role="tablist"
						class="ml-1 flex shrink-0 items-center gap-3"
						aria-label="Which issues"
					>
						{#each issueTabs as tab (tab)}
							<a
								href={linkWith({ tab })}
								role="tab"
								aria-selected={data.tab === tab}
								data-active={data.tab === tab}
								class="relative inline-flex h-7.5 items-center gap-1.5 text-md font-medium whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-transparent after:transition-colors after:duration-110 after:ease-out hover:text-ink-900 hover:after:bg-line-strong data-[active=true]:text-ink-900 data-[active=true]:after:bg-primary"
							>
								{tabLabels[tab]}
								{#if counts}
									<span class="font-mono text-2xs text-muted-foreground tabular-nums">
										{counts[tab]}
									</span>
								{/if}
							</a>
						{/each}
					</div>
				{/if}

				{#if openCycle && progress}
					<span class="hidden shrink-0 items-center gap-2 whitespace-nowrap lg:inline-flex">
						<span class="font-mono text-xs text-muted-foreground">
							{cycleWindow(openCycle.startsOn, openCycle.endsOn)}
						</span>
						<ProgressBar {progress} />
					</span>
				{/if}
			</div>

			<div class="flex flex-none items-center gap-1">
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
				<Button href={at("/inbox")} variant="outline" size="icon-sm" aria-label="Notifications">
					<Bell class="size-icon-toolbar" aria-hidden="true" />
				</Button>
				<Button size="sm" disabled={!team} onclick={() => raise()}>
					<Plus aria-hidden="true" />
					New issue
				</Button>
			</div>
		</div>

		<div
			class="flex min-h-8.5 flex-wrap items-center gap-x-3 gap-y-1 border-t border-line-subtle py-1 pr-3 pl-3.5"
		>
			<PropertyPicker
				options={facetOptions}
				bind:open={filterOpen}
				bind:search={filterSearch}
				placeholder={filterCategory
					? `Filter by ${facetLabels[filterCategory].toLowerCase()}…`
					: "Filter by property or value…"}
				class="w-59"
				empty="No matching property"
				onpick={pickFacet}
			>
				{#snippet trigger(props)}
					<Button {...props} variant="ghost" size="sm" class="shrink-0">
						<Funnel aria-hidden="true" />
						Filter
					</Button>
				{/snippet}
				{#snippet shortcut()}
					<Kbd keys="F" />
				{/snippet}
				{#snippet mark(option)}
					{#if !filterCategory && filterSearch.trim() === ""}
						{#if option.value === "state"}
							<StatusIcon category="not_started" decorative />
						{:else if option.value === "assignee"}
							<Users class="text-muted-foreground" aria-hidden="true" />
						{:else if option.value === "priority"}
							<PriorityIcon priority="high" />
						{:else if option.value === "label"}
							<Tags class="text-muted-foreground" aria-hidden="true" />
						{:else if option.value === "project"}
							<Folder class="text-muted-foreground" aria-hidden="true" />
						{:else}
							<CalendarDays class="text-muted-foreground" aria-hidden="true" />
						{/if}
					{:else if option.value === ""}
						<ArrowLeft class="text-muted-foreground" aria-hidden="true" />
					{/if}
				{/snippet}
			</PropertyPicker>

			{#each chips as chip (chip.kind)}
				<span
					class="inline-flex shrink-0 items-center gap-1.5 border-b-2 border-line-strong pb-0.5 text-sm whitespace-nowrap text-foreground"
				>
					{chip.label}
					<a
						href={linkWith({ [chip.kind]: null })}
						aria-label="Remove the {facetLabels[chip.kind].toLowerCase()} filter"
						class="text-muted-foreground hover:text-ink-900"
					>
						<X class="size-3" aria-hidden="true" />
					</a>
				</span>
			{/each}

			{#if facetCount(facets) > 1}
				<a href={cleared} class="shrink-0 text-sm text-muted-foreground hover:text-foreground">
					Clear all
				</a>
			{/if}

			{#if applied.kind === "applied"}
				{#each applied.references as reference (reference.field + reference.value)}
					<span
						data-missing={reference.state !== "resolved"}
						class="shrink-0 rounded-sm border border-line-default px-1.5 py-0.5 text-sm whitespace-nowrap text-muted-foreground data-[missing=true]:border-warning data-[missing=true]:text-warning"
					>
						{referenceLabel(reference)}
					</span>
				{/each}
				<a
					href={linkWith({ view: null })}
					class="shrink-0 text-sm whitespace-nowrap text-link underline-offset-2 hover:text-link-hover hover:underline"
				>
					Clear view
				</a>
				<a
					href={viewsPath(slug)}
					class="shrink-0 text-sm whitespace-nowrap text-link underline-offset-2 hover:text-link-hover hover:underline"
				>
					Manage views
				</a>
			{:else}
				<button
					type="button"
					onclick={() => (saving = true)}
					class="shrink-0 cursor-pointer text-sm whitespace-nowrap text-link underline-offset-2 hover:text-link-hover hover:underline"
				>
					Save as view
				</button>
			{/if}

			<div class="min-w-2 flex-1"></div>

			<div class="flex shrink-0 items-center gap-1">
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="sm" class="shrink-0">
							Grouped by {groupingNouns[display.grouping]}
							<ChevronDown aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.Label>Group by</DropdownMenu.Label>
					{#each groupings as grouping (grouping)}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a
									href={linkWith({ group: grouping === "state" ? null : grouping })}
									{...props}
								>
									<span class="flex-1">{groupingLabels[grouping]}</span>
									{#if display.grouping === grouping}
										<span class="font-mono text-2xs text-ink-600">✓</span>
									{/if}
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/each}
				</DropdownMenu.Content>
			</DropdownMenu.Root>

			<Popover.Root bind:open={displayOpen}>
				<Popover.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="outline"
							size="icon-sm"
							aria-label="Display options"
							class={atDefaults(display)
								? ""
								: "border-primary bg-primary text-primary-foreground hover:border-primary-active hover:bg-primary-active hover:text-primary-foreground"}
						>
							<Settings class="size-icon-toolbar" aria-hidden="true" />
						</Button>
					{/snippet}
				</Popover.Trigger>
				<Popover.Content align="end" class="w-69">
					{#if displayPane === "root"}
						<div class="flex flex-col p-3">
							<button
								type="button"
								onclick={() => (displayPane = "grouping")}
								class="flex h-7.5 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
							>
								<span class="text-md text-foreground">Grouping</span>
								<span class="flex-1"></span>
								<span class="text-sm text-muted-foreground">{groupingLabels[display.grouping]}</span>
								<ChevronRight class="size-3.25 text-muted-foreground" aria-hidden="true" />
							</button>

							<button
								type="button"
								onclick={() => (displayPane = "ordering")}
								class="flex h-7.5 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
							>
								<span class="text-md text-foreground">Ordering</span>
								<span class="flex-1"></span>
								<span class="text-sm text-muted-foreground">{orderingLabels[display.ordering]}</span>
								<ChevronRight class="size-3.25 text-muted-foreground" aria-hidden="true" />
							</button>

							<label class="flex h-7.5 items-center gap-2 px-1.5 text-md text-foreground">
								<span class="flex-1">Show empty groups</span>
								<Switch
									checked={display.showEmpty}
									onCheckedChange={() =>
										goto(linkWith({ empty: display.showEmpty ? null : "1" }), { noScroll: true })}
								/>
							</label>

							<span class="-mx-3 my-2.5 h-px bg-line-subtle" aria-hidden="true"></span>

							<span class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
								Display properties
							</span>
							<div class="flex flex-wrap gap-1.5 pt-2">
								{#each rowProperties as property (property)}
									<a
										href={linkWith({ hide: hiddenParam(display.shown, property) })}
										data-on={display.shown.includes(property)}
										class="inline-flex h-5.5 items-center rounded-sm border border-line-default bg-card px-2 text-xs font-medium text-ink-600 transition-colors duration-110 ease-out hover:text-ink-900 data-[on=true]:border-primary data-[on=true]:bg-primary data-[on=true]:text-primary-foreground"
									>
										{rowPropertyLabels[property]}
									</a>
								{/each}
							</div>

							<span class="-mx-3 mt-3 mb-2 h-px bg-line-subtle" aria-hidden="true"></span>

							<a
								href={linkWith({ group: null, order: null, empty: null, hide: null })}
								class="text-sm text-muted-foreground hover:text-foreground"
							>
								Reset to defaults
							</a>
						</div>
					{:else}
						<div class="flex flex-col p-1.5">
							<button
								type="button"
								onclick={() => (displayPane = "root")}
								class="flex h-7 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
							>
								<ArrowLeft class="size-3.25 text-muted-foreground" aria-hidden="true" />
								<span class="text-sm text-muted-foreground">
									{displayPane === "grouping" ? "Grouping" : "Ordering"}
								</span>
							</button>

							<span class="-mx-1.5 my-1 h-px bg-line-subtle" aria-hidden="true"></span>

							{#if displayPane === "grouping"}
								{#each groupings as grouping (grouping)}
									<a
										href={linkWith({ group: grouping === "state" ? null : grouping })}
										class="flex h-7 items-center gap-2 rounded-sm px-1.5 text-md text-foreground hover:bg-accent"
									>
										<span class="flex-1">{groupingLabels[grouping]}</span>
										{#if display.grouping === grouping}
											<Check class="size-3.25 text-ink-900" aria-hidden="true" />
										{/if}
									</a>
								{/each}
							{:else}
								{#each orderings as ordering (ordering)}
									<a
										href={linkWith({ order: ordering === "manual" ? null : ordering })}
										class="flex h-7 items-center gap-2 rounded-sm px-1.5 text-md text-foreground hover:bg-accent"
									>
										<span class="flex-1">{orderingLabels[ordering]}</span>
										{#if display.ordering === ordering}
											<Check class="size-3.25 text-ink-900" aria-hidden="true" />
										{/if}
									</a>
								{/each}
							{/if}
						</div>
					{/if}
				</Popover.Content>
			</Popover.Root>
			</div>
		</div>
	</div>

	<div class="relative flex min-h-0 flex-1 flex-col">
		{#if saving}
			<div class="px-4 pt-3">
				<form onsubmit={saveView} class="flex flex-wrap items-end gap-2">
					<div class="flex min-w-0 flex-[1_1_200px] flex-col gap-1">
						<label for="save-view-name" class="text-sm text-muted-foreground">
							Save what you are looking at, for yourself
						</label>
						<Input
							id="save-view-name"
							bind:value={viewName}
							disabled={savingView}
							placeholder="Urgent and unassigned"
							class="h-7.5"
						/>
					</div>
					<Button type="submit" size="sm" disabled={savingView || !viewName.trim()}>
						{savingView ? "Saving" : "Save view"}
					</Button>
					<Button
						type="button"
						variant="secondary"
						size="sm"
						disabled={savingView}
						onclick={() => (saving = false)}
					>
						Cancel
					</Button>
				</form>
			</div>
		{/if}

		{#if applied.kind === "gone"}
			<div class="px-4 pt-3">
				<Alert.Root>
					<CircleX aria-hidden="true" />
					<Alert.Title>That view is gone</Alert.Title>
					<Alert.Description>
						Someone removed it, or stopped sharing it with you.
						<a href={linkWith({ view: null })} class="text-link underline-offset-2 hover:underline">
							Show all issues instead
						</a>.
					</Alert.Description>
				</Alert.Root>
			</div>
		{/if}

		{#if broken.length > 0}
			<div class="px-4 pt-3">
				<Alert.Root>
					<CircleX aria-hidden="true" />
					<Alert.Title>This view points at something that is gone</Alert.Title>
					<Alert.Description>
						{broken.length === 1 ? "One thing" : `${broken.length} things`} this view filters on no
						longer exists, so it may return less than it used to.
						<a href={viewsPath(slug)} class="text-link underline-offset-2 hover:underline">
							Manage views
						</a>.
					</Alert.Description>
				</Alert.Root>
			</div>
		{/if}

		{#if failure}
			<div class="px-4 pt-3">
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not stick</Alert.Title>
					<Alert.Description>{failure}</Alert.Description>
				</Alert.Root>
			</div>
		{/if}

		{#if board.kind === "unavailable"}
			<div class="my-auto flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">We could not load these issues</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Nothing changed. Wait a moment and try again.
				</p>
			</div>
		{:else if board.kind === "no_teams"}
			<div class="my-auto flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">No teams yet</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Issues belong to a team. Make one first, and it arrives with a set of states ready to use.
				</p>
				<Button href={at("/settings/teams")} variant="secondary" size="sm">Create a team</Button>
			</div>
		{:else if board.kind === "empty" && filtered}
			<div class="my-auto flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">
					No issues match these filters
				</p>
				<Button href={cleared} variant="secondary" size="sm">Clear filters</Button>
			</div>
		{:else if board.kind === "empty"}
			<div class="my-auto flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">
					{data.tab === "backlog"
						? `Nothing in ${board.team}'s backlog`
						: data.tab === "active"
							? `Nothing active in ${board.team}`
							: `Nothing in ${board.team}`}
				</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					{#if applied.kind === "applied"}
						Nothing you can see matches this view right now. Someone else opening it may well see
						something &mdash; it is evaluated against whichever teams each person can see.
					{:else}
						Raise the first one, and it starts in whichever state this team files new work into.
					{/if}
				</p>
				<Button size="sm" disabled={!team} onclick={() => raise()}>
					<Plus aria-hidden="true" />
					New issue
				</Button>
			</div>
		{:else if data.layout === "list"}
			<div class="flex-1 overflow-auto">
				{#each columns as column (column.key)}
					{@const offset = offsets.get(column.key) ?? 0}
					{@const shut = collapsed.has(column.key)}
					<section
						aria-label={column.name}
						ondragover={(event) => onDragOver(event, column.key)}
						ondrop={(event) => onDrop(event, column.key)}
						data-dropping={dropTarget === column.key}
						class="data-[dropping=true]:bg-accent"
					>
						<div
							class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-1.5"
						>
							<button
								type="button"
								aria-expanded={!shut}
								onclick={() =>
									shut ? collapsed.delete(column.key) : collapsed.add(column.key)}
								class="inline-flex h-6 cursor-pointer items-center gap-1.5 rounded-sm px-1 hover:bg-accent"
							>
								<ChevronDown
									class="size-3.25 text-muted-foreground transition-transform duration-110 ease-out {shut
										? '-rotate-90'
										: ''}"
									aria-hidden="true"
								/>
								{@render columnMark(column)}
								<span
									class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
								>
									{column.name}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{column.total}
								</span>
							</button>
							<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
							<Button
								variant="ghost"
								size="icon-xs"
								aria-label="New issue in {column.name}"
								disabled={!team}
								onclick={() => raise(seedFor(column.key))}
							>
								<Plus aria-hidden="true" />
							</Button>
						</div>
						{#if !shut}
							{#each column.issues as issue, index (issue.id)}
								<IssueRow
									{issue}
									href={at(`/issues/${issue.reference}`)}
									assignee={names.get(issue.assigneeAccountId ?? "") ?? ""}
									now={data.now}
									timezone={data.workspace.timezone}
									shown={display.shown}
									cursor={offset + index === cursor}
									selected={selected.has(issue.id)}
									onselect={(extend) => toggle(issue.id, extend)}
									{priorityControl}
									{stateControl}
									{labelsControl}
									{assigneeControl}
									draggable={display.grouping === "state"}
									dragging={dragging === issue.id}
									ondragstart={(event) => onDragStart(event, issue.id)}
									ondragend={onDragEnd}
								/>
							{/each}
							<ColumnMore
								load={column.load}
								name={column.name}
								layout="list"
								onload={() => void loadColumn(column)}
							/>
						{/if}
					</section>
				{/each}
			</div>
		{:else}
			<div class="flex-1 overflow-auto bg-background p-4">
				<div class="flex min-h-full items-start gap-3">
					{#each columns as column (column.key)}
						<div
							role="group"
							aria-label={column.name}
							ondragover={(event) => onDragOver(event, column.key)}
							ondrop={(event) => onDrop(event, column.key)}
							data-dropping={dropTarget === column.key}
							class="group/column flex w-60 flex-none sm:w-62.5 flex-col gap-2 rounded-lg border border-transparent p-1 transition-colors duration-70 ease-out data-[dropping=true]:border-dashed data-[dropping=true]:border-ink-400 data-[dropping=true]:bg-accent"
						>
							<div class="flex h-7 items-center gap-2 px-1">
								{@render columnMark(column)}
								<span
									class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
								>
									{column.name}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{column.total}
								</span>
								<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
								<Button
									variant="ghost"
									size="icon-xs"
									aria-label="New issue in {column.name}"
									disabled={!team}
									onclick={() => raise(seedFor(column.key))}
								>
									<Plus aria-hidden="true" />
								</Button>
							</div>

							{#each column.issues as issue (issue.id)}
								<IssueCard
									{issue}
									href={at(`/issues/${issue.reference}`)}
									assignee={names.get(issue.assigneeAccountId ?? "") ?? ""}
									now={data.now}
									timezone={data.workspace.timezone}
									shown={display.shown}
									selected={selected.has(issue.id)}
									onselect={(extend) => toggle(issue.id, extend)}
									{priorityControl}
									{stateControl}
									{labelsControl}
									{assigneeControl}
									draggable={display.grouping === "state"}
									dragging={dragging === issue.id}
									ondragstart={(event) => onDragStart(event, issue.id)}
									ondragend={onDragEnd}
								/>
							{/each}

							{#if column.issues.length === 0 && column.load.kind === "complete"}
								<p
									class="border-t border-dashed border-line-strong px-0.5 py-3 font-mono text-2xs tracking-eyebrow text-ink-600 uppercase"
								>
									Nothing here
								</p>
							{/if}

							<ColumnMore
								load={column.load}
								name={column.name}
								layout="board"
								onload={() => void loadColumn(column)}
							/>

							<button
								type="button"
								disabled={!team}
								onclick={() => raise(seedFor(column.key))}
								class="flex h-7.5 w-full cursor-pointer items-center gap-1.75 rounded-md border border-dashed border-line-strong px-2 text-sm text-ink-600 opacity-0 transition-opacity duration-70 ease-out group-hover/column:opacity-100 focus-visible:opacity-100 hover:bg-accent hover:text-foreground disabled:pointer-events-none"
							>
								<Plus class="size-3.5" aria-hidden="true" />
								New issue
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if toast}
			<div class="absolute right-3.5 bottom-3.5 z-10">
				<Toast message={toast.message} onaction={toast.undo && (() => void toast?.undo?.())} />
			</div>
		{/if}
	</div>

	{#if bulk}
		<div class="flex-none border-t border-line-default px-4 py-2">
			<BulkResult result={bulk} />
		</div>
	{/if}

	{#if selected.size > 0}
		<BulkBar
			count={selected.size}
			states={sharedStates}
			{members}
			cycles={teamCycles}
			working={applying}
			onpriority={(priority) => applyBulk({ priority })}
			onstate={(stateId) => applyBulk({ stateId })}
			oncycle={(cycleId) => applyBulk({ cycleId })}
			onassignee={(accountId) =>
				applyBulk(accountId === "" ? { clearAssignee: true } : { assigneeId: accountId })}
			onstatus={(status) => applyBulk({ status })}
			onclear={clearSelection}
		/>
	{:else}
		<div
			class="hidden h-8.5 flex-none items-center gap-4 border-t border-line-subtle bg-card pr-3.5 pl-3.5 md:flex"
		>
			<span class="font-mono text-xs text-muted-foreground tabular-nums">
				{total}
				{total === 1 ? "issue" : "issues"}
			</span>
			<span class="flex-1"></span>
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="↑ ↓" />move
			</span>
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="X" />select
			</span>
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="F" />filter
			</span>
			<span class="hidden items-center gap-1.5 text-xs text-muted-foreground lg:inline-flex">
				<Kbd keys="1" />list
			</span>
			<span class="hidden items-center gap-1.5 text-xs text-muted-foreground lg:inline-flex">
				<Kbd keys="2" />board
			</span>
			<span class="hidden items-center gap-1.5 text-xs text-muted-foreground lg:inline-flex">
				<Kbd keys="C" />new issue
			</span>
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="⌘ K" />go anywhere
			</span>
		</div>
	{/if}
</div>

{#if team}
	<NewIssueDialog
		bind:open={creating}
		workspaceId={data.workspace.id}
		teams={data.teams}
		states={statesByTeam}
		{members}
		{labels}
		projects={data.projects ?? []}
		today={data.today}
		{prefill}
		oncreated={(issue) => {
			void invalidate(keys.page(page.route.id));
			announce(`Created ${issue.reference}`);
		}}
	/>
{/if}
