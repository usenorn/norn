<script lang="ts">
	import { tick, untrack } from "svelte";
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleHelp from "@lucide/svelte/icons/circle-help";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Folder from "@lucide/svelte/icons/folder";
	import Kanban from "@lucide/svelte/icons/kanban";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Plus from "@lucide/svelte/icons/plus";
	import { SvelteSet } from "svelte/reactivity";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import PersonAvatar from "$lib/components/norn/person-avatar.svelte";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import LabelPicker from "$lib/labels/label-picker.svelte";
	import { setIssueLabels, type LabelsOutcome } from "$lib/labels/set-issue-labels";
	import type { Label } from "$lib/labels/labels";
	import BulkBar, { type BulkPicker } from "$lib/issues/bulk-bar.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { bindShortcuts, useShortcuts } from "$lib/shortcuts/registry.svelte";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import { showFailure, showToast } from "$lib/toast/toasts";
	import { announceCreated } from "./created-toast";
	import { attempt, outcomeLine, unknownLine, type Outcome } from "$lib/api/attempt";
	import { statusIndexOf } from "$lib/issues/set-status";
	import BulkResult from "$lib/issues/bulk-result.svelte";
	import IssueCard from "$lib/issues/issue-card.svelte";
	import { registerNewIssue, useNewIssue } from "$lib/issues/new-issue.svelte";
	import { registerCommandTargets } from "$lib/command/scope.svelte";
	import PropertyPicker from "$lib/issues/property-picker.svelte";
	import DisplayMenu from "$lib/issues/display-menu.svelte";
	import FilterBar from "$lib/issues/filter-bar.svelte";
	import {
		clearedLink,
		dueEntries,
		unassignedEntry,
		type FacetCatalogue,
	} from "$lib/issues/facet-options";
	import { linkTo } from "$lib/issues/linking";
	import { withReturn } from "$lib/issues/return-to-list";
	import { failures, rangeBetween, settled, type BulkActionResult } from "$lib/issues/bulk";
	import { api } from "$lib/api";
	import { flash } from "$lib/motion";
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
		boardGroupings,
		surfaceDefaults,
		surfaceOrderings,
		groupings,
		issueTabs,
		orderings,
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
	import { withEdit, without, type PendingEdit } from "$lib/issues/pending";
	import type { NewIssuePrefill } from "$lib/issues/new-issue-schema";
	import {
		settledWith,
		unsettled,
		withDraft,
		type CreationOutcome,
		type IssueCreation,
	} from "$lib/issues/creating";
	import {
		changedProperty,
		insertionIndex,
		landing,
		movedInto,
		stayedPut,
		type DropTarget,
		type PendingMove,
	} from "$lib/issues/drop";
	import { expectedVersion, remember } from "$lib/issues/versions";
	import {
		columnFilter,
		facetCount,
		pickableFacets,
		unassigned,
	} from "$lib/issues/facets";
	import { issueFailureMessage, priorities, priorityLabel, readIssueFailure } from "$lib/issues/issues";
	import type { IssuePriority } from "$lib/issues/issues";
	import { brokenIn } from "$lib/views/applied";
	import { referenceLabel, scopeOf, viewsPath } from "$lib/views/views";
	import type { WorkflowState } from "$lib/team/states";
	import { assignees } from "$lib/workspace/members";
	import { workspacePath } from "$lib/workspace/navigation";
	import { cycleWindow } from "$lib/time";
	import { cyclingTeams, openCycles, type TeamCyclesRead } from "$lib/cycles/cycles";
	import type { IssuesListingData, IssuesListingScope, IssuesPreview } from "./listing";
	import Retry from "$lib/components/norn/retry.svelte";
	import { Pending } from "$lib/api/pending.svelte";
	import { Watch } from "$lib/api/watch.svelte";

	let {
		data,
		basePath,
		preview,
	}: {
		data: IssuesListingData & IssuesListingScope;
		basePath: string;
		preview?: IssuesPreview;
	} = $props();


	const slug = $derived(data.workspace.slug);
	const at = $derived((path: string) => workspacePath(slug, path));

	let pages = $state.raw<BoardPages>(noPages);
	let moves = $state.raw<PendingMove[]>([]);
	let edits = $state.raw<PendingEdit[]>([]);
	let creations = $state.raw<IssueCreation[]>([]);

	let dragging = $state<string | null>(null);
	let dropTarget = $state<DropTarget | null>(null);
	let viewFailure = $state<string | null>(null);
	const rowPending = new Pending();

	const team = $derived(preview?.team ?? data.team);
	const states = $derived(preview?.states ?? data.states ?? []);
	const progress = $derived(preview?.progress ?? data.progress);
	const members = $derived(preview?.members ?? data.members ?? []);
	const people = $derived(assignees(members));
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

	const signature = $derived(JSON.stringify(data.query));
	const held = $derived(preview?.pages ?? pagesOf(pages, signature));

	$effect(() => {
		base;
		moves = [];
		edits = [];
		creations = unsettled(untrack(() => creations));
	});

	const arriving = $derived(creations.map((creation) => creation.issue));
	const draftIDs = $derived(new Set(unsettled(creations).map((creation) => creation.issue.id)));

	const applied = $derived(preview?.applied ?? data.applied);
	const teams = $derived(preview?.teams ?? data.teams);
	const broken = $derived(brokenIn(applied));
	const scopeName = $derived(
		applied.kind === "applied" ? applied.view.name : (team?.name ?? data.workspace.name)
	);

	const teamCycles = $derived(
		(data.cycles ?? []).filter((entry) => !team || entry.teamId === team.id).map((entry) => entry.cycle)
	);
	const openCycle = $derived(teamCycles.find((cycle) => cycle.id === facets.cycle));
	const chosenProject = $derived(
		(data.projects ?? []).find((project) => project.id === facets.project)
	);

	const backlog = $derived(backlogStates(states));

	const board = $derived(
		boardFor(
			source,
			display.grouping,
			{
				states,
				members: people,
				projects: data.projects ?? [],
				tab: data.tab,
				backlogStateIds: backlog.map((state) => state.id),
			},
			held,
			{ name: scopeName, teams: teams.length },
			{ showEmpty: display.showEmpty, moves, edits, creations: arriving }
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
	const keyOfTeam = $derived((teamId: string) => teams.find((each) => each.id === teamId)?.key);
	const ambiguous = $derived(new Set(states.map((state) => state.teamId)).size > 1);
	const stateLabel = $derived((state: WorkflowState) =>
		ambiguous ? `${state.name} · ${keyOfTeam(state.teamId) ?? ""}`.trim() : state.name
	);
	const columnName = $derived((column: (typeof columns)[number]) =>
		ambiguous && column.mark.kind === "state" ? stateLabel(column.mark.state) : column.name
	);
	const counts = $derived(tabCounts(preview?.totals ?? data.totals, states, backlog, arriving));
	const counted = $derived(tallyTotal(tallies));
	const total = $derived(counted === undefined ? flat.length : counted + arriving.length);

	const params = $derived.by(() => {
		const q = new URLSearchParams();

		if (applied.kind === "applied") q.set("view", applied.view.id);
		else if (applied.kind === "gone") q.set("view", applied.id);

		for (const [kind, value] of Object.entries(facets)) if (value) q.set(kind, value);

		for (const [key, value] of writeDisplay(display, data.layout, data.tab)) q.set(key, value);

		return q;
	});

	const linkWith = $derived((changes: Record<string, string | null>) =>
		linkTo(basePath, params, changes)
	);

	const cleared = $derived(clearedLink(pickableFacets, linkWith));

	const returnTo = $derived(linkWith({}));
	const issueHref = $derived((reference: string) =>
		withReturn(at(`/issues/${reference}`), returnTo)
	);

	const catalogue = $derived<FacetCatalogue>({
		state: states.map((state) => ({ value: state.id, label: stateLabel(state) })),
		assignee: [
			...people.map((member) => ({
				value: member.accountId,
				label: member.displayName ?? "Someone",
			})),
			unassignedEntry(),
		],
		priority: priorities.map((entry) => ({ value: entry.value, label: entry.label })),
		label: labels.map((label) => ({ value: label.id, label: label.name, color: label.color })),
		project: (data.projects ?? []).map((project) => ({ value: project.id, label: project.name })),
		cycle: teamCycles.map((cycle) => ({ value: cycle.id, label: cycle.name })),
		due: dueEntries(),
	});

	function announce(message: string, undo?: () => Promise<void>, href?: string) {
		showToast(message, { href, onaction: undo && (() => void undo()) });
	}

	function refusedLine(outcome: Outcome<unknown>): string {
		return outcome.kind === "refused"
			? issueFailureMessage(readIssueFailure(outcome.problem))
			: unknownLine;
	}

	async function patch(
		issue: Issue,
		body: Record<string, unknown>,
		options: { reload?: boolean } = {}
	): Promise<Outcome<unknown>> {
		const outcome = await attempt({
			run: () =>
				api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: expectedVersion(issue), ...body },
				}),
		});

		if (outcome.kind === "done") remember(outcome.value);

		if (outcome.kind !== "done") {
			showFailure(refusedLine(outcome), { href: at(`/issues/${issue.reference}`) });
			await invalidate(keys.page(page.route.id));

			return outcome;
		}

		if (options.reload !== false) await invalidate(keys.page(page.route.id));

		return outcome;
	}

	function asLoaded(issue: Issue): Issue {
		return flat.find((candidate) => candidate.id === issue.id) ?? issue;
	}

	function change(
		issue: Issue,
		body: Record<string, unknown>,
		previous: Record<string, unknown>,
		message: string,
		optimistic: Partial<Issue>
	) {
		return rowPending.once(issue.id, () => changing(issue, body, previous, message, optimistic));
	}

	async function changing(
		issue: Issue,
		body: Record<string, unknown>,
		previous: Record<string, unknown>,
		message: string,
		optimistic: Partial<Issue>
	) {
		edits = withEdit(edits, issue.id, optimistic);

		const outcome = await patch(issue, body, { reload: false });

		if (outcome.kind !== "done") {
			edits = without(edits, issue.id);

			return;
		}

		announce(
			message,
			async () => {
				const undone = await patch(asLoaded(issue), previous, { reload: false });

				if (undone.kind !== "done") return;

				edits = without(edits, issue.id);
				await invalidate(keys.page(page.route.id));
			},
			at(`/issues/${issue.reference}`)
		);

		await invalidate(keys.page(page.route.id));
	}

	async function setState(issue: Issue, stateId: string) {
		if (issue.state.id === stateId) return;

		const target = states.find((state) => state.id === stateId);

		await change(
			issue,
			{ stateId },
			{ stateId: issue.state.id },
			`Moved ${issue.reference} to ${target?.name ?? "another status"}`,
			target ? { state: target } : {}
		);
	}

	async function setPriority(issue: Issue, priority: IssuePriority) {
		if (issue.priority === priority) return;

		await change(
			issue,
			{ priority },
			{ priority: issue.priority },
			`Set ${issue.reference} to ${priorityLabel(priority).toLowerCase()}`,
			{ priority }
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
				: `Assigned ${issue.reference} to ${name}`,
			{ assigneeAccountId: accountId || undefined }
		);
	}

	async function changeLabels(issue: Issue, labelIds: string[], picked: Label) {
		const held = issue.labels.map((label) => label.id);
		const adding = labelIds.includes(picked.id);

		const outcome = await setLabels(issue, labelIds, () =>
			withEdit(edits, issue.id, {
				labels: labelIds.flatMap(
					(id) => [picked, ...labels].find((label) => label.id === id) ?? []
				),
			})
		);

		if (outcome.kind !== "changed") return;

		announce(
			adding
				? `Added ${picked.name} to ${issue.reference}`
				: `Removed ${picked.name} from ${issue.reference}`,
			async () => {
				const undone = await setLabels(asLoaded(issue), held);

				if (undone.kind !== "changed") return;

				edits = without(edits, issue.id);
				await invalidate(keys.page(page.route.id));
			},
			at(`/issues/${issue.reference}`)
		);

		await invalidate(keys.page(page.route.id));
	}

	async function setLabels(
		issue: Issue,
		labelIds: string[],
		optimistic?: () => PendingEdit[]
	): Promise<LabelsOutcome> {
		const held = edits;

		const outcome = await setIssueLabels({
			workspaceId: data.workspace.id,
			issue,
			labelIds,
			optimistic: optimistic && (() => (edits = optimistic())),
			reconcile: () => (edits = held),
		});

		if (outcome.kind === "refused" || outcome.kind === "uncertain") {
			showFailure(
				outcome.kind === "refused"
					? issueFailureMessage(readIssueFailure(outcome.problem))
					: unknownLine,
				{ href: at(`/issues/${issue.reference}`) }
			);
			await invalidate(keys.page(page.route.id));
		}

		return outcome;
	}

	async function loadColumn(column: IssueColumn) {
		if (column.load.kind === "complete" || column.load.kind === "loading") return;

		const from = signature;

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

		if (!issue) return false;
		if (display.grouping !== "state") return true;

		const target = states.find((state) => state.id === key);

		return !target || target.teamId === issue.teamId;
	}

	function onDragOver(event: DragEvent, key: string) {
		event.preventDefault();

		if (!dragging || !landsIn(dragging, key)) {
			if (event.dataTransfer) event.dataTransfer.dropEffect = "none";

			return;
		}

		if (event.dataTransfer) event.dataTransfer.dropEffect = "move";

		dropTarget = {
			key,
			index: insertionIndex(event.currentTarget as Element, event.clientY),
		};
	}

	function onDragLeave(event: DragEvent, key: string) {
		const leaving = event.currentTarget as Element;

		if (leaving.contains(event.relatedTarget as Node)) return;
		if (dropTarget?.key === key) dropTarget = null;
	}

	async function onDrop(event: DragEvent, key: string) {
		event.preventDefault();

		const id = event.dataTransfer?.getData("text/plain") || dragging;
		const target = dropTarget ?? {
			key,
			index: insertionIndex(event.currentTarget as Element, event.clientY),
		};

		onDragEnd();

		const issue = flat.find((candidate) => candidate.id === id);
		if (!issue || !landsIn(id, target.key)) return;

		const held = columns.find((candidate) => candidate.key === target.key)?.issues ?? [];

		if (stayedPut(issue, held, target, display.grouping)) return;

		const placed = landing(issue, held, target, display.grouping, display.ordering);

		if (!movedInto(placed.move)) return;

		if (rowPending.busy(issue.id)) return;

		moves = [...moves.filter((held) => held.issueId !== issue.id), placed.pending];

		const outcome = await rowPending.once(issue.id, () =>
			patch(issue, placed.move, { reload: false })
		);

		if (!outcome) return;

		if (outcome.kind !== "done") {
			moves = moves.filter((held) => held.issueId !== issue.id);

			return;
		}

		if (changedProperty(placed.move)) {
			announce(
				movedMessage(issue, target.key),
				async () => {
					const undone = await patch(asLoaded(issue), returning(issue, placed), {
						reload: false,
					});

					if (undone.kind !== "done") return;

					moves = moves.filter((held) => held.issueId !== issue.id);
					await invalidate(keys.page(page.route.id));
				},
				at(`/issues/${issue.reference}`)
			);
		}

		await invalidate(keys.page(page.route.id));
	}

	function movedMessage(issue: Issue, key: string): string {
		const name = columns.find((column) => column.key === key)?.name ?? "another column";

		switch (display.grouping) {
			case "priority":
				return key === "none"
					? `Cleared the priority on ${issue.reference}`
					: `Set ${issue.reference} to ${priorityLabel(key as IssuePriority).toLowerCase()} priority`;
			case "assignee":
				return key ? `Assigned ${issue.reference} to ${name}` : `Unassigned ${issue.reference}`;
			case "project":
				return key
					? `Put ${issue.reference} in ${name}`
					: `Took ${issue.reference} out of its project`;
			default:
				return `Moved ${issue.reference} to ${name}`;
		}
	}

	function returning(issue: Issue, placed: ReturnType<typeof landing>): Record<string, unknown> {
		const back: Record<string, unknown> = {};
		const cleared: string[] = [];
		const touched = (field: string) =>
			placed.move.clear?.includes(field) ?? false;

		if (placed.move.stateId) back.stateId = issue.state.id;
		if (placed.move.priority) back.priority = issue.priority;

		if (placed.move.dueOn || touched("dueOn")) {
			if (issue.dueOn) back.dueOn = issue.dueOn;
			else cleared.push("dueOn");
		}

		if (placed.move.assigneeId || touched("assignee")) {
			if (issue.assigneeAccountId) back.assigneeId = issue.assigneeAccountId;
			else cleared.push("assignee");
		}

		if (placed.move.projectId || touched("project")) {
			if (issue.projectId) back.projectId = issue.projectId;
			else cleared.push("project");
		}

		if (cleared.length > 0) back.clear = cleared;

		return back;
	}

	let saving = $state(false);
	let viewName = $state("");
	let savingView = $state(false);

	async function saveView(event: SubmitEvent) {
		event.preventDefault();

		if (!viewName.trim()) return;

		savingView = true;
		viewFailure = null;

		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/saved-views", {
					params: { path: { workspaceId: data.workspace.id } },
					body: {
						name: viewName.trim(),
						sharing: "personal",
						filter: data.query.filter,
						sort: data.query.sort,
						groupBy: data.query.groupBy,
					},
				}),
		});

		savingView = false;

		if (outcome.kind !== "done") {
			viewFailure = outcomeLine(outcome, "That view was not saved. Nothing changed — try again.");

			return;
		}

		viewName = "";
		saving = false;
		await invalidate(keys.page(page.route.id));
	}

	let selected = $state(new SvelteSet<string>());
	let anchor = $state<string | null>(null);
	let applying = $state(false);
	let asked = $state.raw<{ change: Record<string, unknown>; issueIds: string[] } | null>(null);

	const bulkWatch = new Watch<BulkActionResult>({
		read: (bulkActionId) =>
			api.GET("/workspaces/{workspaceId}/bulk-actions/{bulkActionId}", {
				params: { path: { workspaceId: data.workspace.id, bulkActionId } },
			}),
		settled: (result) => settled(result.status),
		identify: (result) => result.id,
	});

	let concluded = "";

	$effect(() => () => bulkWatch.stop());

	$effect(() => {
		const result = bulkWatch.value;

		if (!result || !settled(result.status) || concluded === result.id) return;

		concluded = result.id;

		void conclude();
	});
	let collapsed = $state(new SvelteSet<string>());
	let filterOpen = $state(false);
	let displayOpen = $state(false);
	let displayPane = $state<"root" | "grouping" | "ordering">("root");

	$effect(() => {
		if (!displayOpen) displayPane = "root";
	});

	const bulk = $derived(preview?.bulk ?? bulkWatch.value ?? null);
	const orderedIDs = $derived(
		flat.filter((issue) => !draftIDs.has(issue.id)).map((issue) => issue.id)
	);

	const selectedTeams = $derived(
		new Set(flat.filter((issue) => selected.has(issue.id)).map((issue) => issue.teamId))
	);
	const sharedStates = $derived(
		selectedTeams.size === 1 ? statesOfTeam([...selectedTeams][0]) : []
	);
	const cycling = $derived(cyclingTeams(data.cycles ?? []));
	const selectedTeam = $derived(selectedTeams.size === 1 ? [...selectedTeams][0] : null);

	let selectionCycles = $state.raw<TeamCyclesRead | null>(null);

	const sharedCycles = $derived.by((): TeamCyclesRead | null => {
		if (!selectedTeam || !cycling.has(selectedTeam)) return null;

		return selectionCycles?.teamId === selectedTeam
			? selectionCycles
			: { kind: "loading", teamId: selectedTeam };
	});

	$effect(() => {
		const teamId = selectedTeam;

		if (!teamId || !cycling.has(teamId) || untrack(() => selectionCycles?.teamId) === teamId) return;

		void loadCycles(teamId);
	});

	async function loadCycles(teamId: string) {
		selectionCycles = { kind: "loading", teamId };

		const read = await api
			.GET("/workspaces/{workspaceId}/cycles", {
				params: { path: { workspaceId: data.workspace.id }, query: { teamId } },
			})
			.catch(() => undefined);

		if (selectionCycles?.teamId !== teamId) return;

		selectionCycles = read?.data
			? { kind: "ready", teamId, cycles: openCycles(read.data, teamId) }
			: { kind: "failed", teamId };
	}

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
		asked = null;
		bulkWatch.stop();
	}

	async function conclude() {
		selected.clear();
		anchor = null;

		await invalidate(keys.page(page.route.id));
		await markChanged(asked?.issueIds ?? []);
	}

	async function markChanged(issueIds: string[]) {
		await tick();

		for (const id of issueIds) {
			flash(document.querySelector(`[data-issue="${id}"]`));
		}
	}

	function applyBulk(change: Record<string, unknown>) {
		return send(change, [...selected]);
	}

	function retryFailed() {
		const result = bulk;

		if (!result || !asked) return;

		const again = failures(result).map((outcome) => outcome.issueId);

		return send(asked.change, again);
	}

	async function send(change: Record<string, unknown>, issueIds: string[]) {
		if (issueIds.length === 0) return;

		applying = true;
		concluded = "";
		bulkWatch.stop();

		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/issues/bulk", {
					params: { path: { workspaceId: data.workspace.id } },
					body: { change, issueIds },
				}),
		});

		applying = false;

		if (outcome.kind !== "done") {
			showFailure(refusedLine(outcome));

			return;
		}

		asked = { change, issueIds };

		bulkWatch.start(outcome.value.id, outcome.value);
	}

	async function settle(outcome: CreationOutcome) {
		creations = settledWith(creations, outcome);

		if (outcome.kind === "refused") {
			announce(outcome.failure);

			if (outcome.input) raising.raise(outcome.input);

			return;
		}

		announceCreated(outcome.issue, at(`/issues/${outcome.issue.reference}`), page.url.origin);

		await invalidate(keys.page(page.route.id));
	}

	function seedFor(key: string): NewIssuePrefill {
		switch (display.grouping) {
			case "state":
				return { stateId: key, teamId: states.find((state) => state.id === key)?.teamId };
			case "priority":
				return { priority: key as IssuePriority };
			case "assignee":
				return key ? { assigneeId: key } : {};
			case "project":
				return key ? { projectId: key } : {};
			default:
				return {};
		}
	}

	const shortcuts = useShortcuts();
	const raising = useNewIssue();

	let bulkBar = $state.raw<{ pick: (what: BulkPicker) => void } | undefined>(undefined);

	const cursor = listCursor(() => ({
		rows: flat,
		open: (issue) => {
			if (!draftIDs.has(issue.id)) void goto(issueHref(issue.reference));
		},
		left: () => step(-1),
		right: () => step(1),
	}));

	registerCommandTargets("selection", () => ({
		issues: flat
			.filter((issue) =>
				selected.size > 0 ? selected.has(issue.id) : issue === cursor.row && !draftIDs.has(issue.id)
			)
			.map((issue) => ({
				id: issue.id,
				reference: issue.reference,
				title: issue.title,
				teamId: issue.teamId,
			})),
		onapplied: clearSelection,
	}));

	registerNewIssue(() => ({
		seed: {
			teamId: team?.id ?? "",
			...(chosenProject ? { projectId: chosenProject.id } : {}),
		},
		onraising: (key, draft) => (creations = withDraft(creations, key, draft)),
		onsettled: settle,
	}));

	bindShortcuts({
		"select-toggle": () => {
			const issue = cursor.row;

			if (issue && !draftIDs.has(issue.id)) toggle(issue.id);
		},
		"issue-filter": () => (filterOpen = true),
		"issue-list": () => void goto(linkWith({ layout: null })),
		"issue-board": () => void goto(linkWith({ layout: "board" })),
		"status-set": (binding) => void pickStatus(statusIndexOf(binding)),
	});

	async function pickStatus(nth: number) {
		if (selected.size > 0) {
			const shared = sharedStates[nth];

			if (shared) await applyBulk({ stateId: shared.id });

			return;
		}

		const issue = cursor.row;

		if (!issue || draftIDs.has(issue.id)) return;

		const state = statesOfTeam(issue.teamId)[nth];

		if (state) await setState(issue, state.id);
	}

	function step(by: number): boolean {
		const here = columns.findIndex((column) => {
			const start = offsets.get(column.key) ?? 0;

			return cursor.at >= start && cursor.at < start + column.issues.length;
		});
		const row = here < 0 ? 0 : cursor.at - (offsets.get(columns[here].key) ?? 0);

		for (let next = (here < 0 ? 0 : here) + by; next >= 0 && next < columns.length; next += by) {
			const landing = columns[next];

			if (landing.issues.length === 0) continue;

			cursor.to((offsets.get(landing.key) ?? 0) + Math.min(row, landing.issues.length - 1));

			return true;
		}

		return false;
	}

	$effect(() => {
		if (data.layout !== "board" || cursor.row === undefined) return;

		const released = [
			shortcuts.register("cursor-left", () => step(-1)),
			shortcuts.register("cursor-right", () => step(1)),
		];

		return () => released.forEach((release) => release());
	});

	$effect(() => {
		if (selected.size === 0) return;

		const released = [
			shortcuts.register("select-clear", clearSelection),
			shortcuts.register("bulk-status", () => bulkBar?.pick("state")),
			shortcuts.register("bulk-assignee", () => bulkBar?.pick("assignee")),
			shortcuts.register("bulk-priority", () => bulkBar?.pick("more")),
		];

		return () => released.forEach((release) => release());
	});

	$effect(() => {
		if (selected.size > 0) {
			if (sharedCycles === null) return;

			return shortcuts.register("bulk-cycle", () => bulkBar?.pick("cycle"));
		}

		const issue = cursor.row;

		if (!issue || draftIDs.has(issue.id) || !cycling.has(issue.teamId)) return;

		return shortcuts.register("bulk-cycle", () => void pickCycleFor(issue.id));
	});

	async function pickCycleFor(id: string) {
		toggle(id);
		await tick();
		bulkBar?.pick("cycle");
	}

	const filtered = $derived(facetCount(facets) > 0);
</script>

{#snippet dropGap(key: string, index: number)}
	<div
		aria-hidden="true"
		data-open={dropTarget?.key === key && dropTarget.index === index}
		class="pointer-events-none -my-0.5 h-1 rounded-full bg-primary opacity-0 motion-control data-[open=true]:opacity-100"
	></div>
{/snippet}

{#snippet columnMark(column: (typeof columns)[number])}
	{#if column.mark.kind === "state"}
		<StatusIcon category={column.mark.state.category} name={column.mark.state.name} />
		{#if ambiguous}
			<TeamKey key={keyOfTeam(column.mark.state.teamId) ?? ""} />
		{/if}
	{:else if column.mark.kind === "priority"}
		<PriorityIcon priority={column.mark.priority} />
	{:else if column.mark.kind === "assignee"}
		{#if column.mark.name}
			<PersonAvatar
				accountId={column.mark.accountId}
				name={column.mark.name}
				size="xs"
				title={column.mark.name}
			/>
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
				disabled={draftIDs.has(issue.id)}
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
				disabled={draftIDs.has(issue.id)}
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
	<LabelPicker
		workspaceId={data.workspace.id}
		{labels}
		chosen={issue.labels}
		chips={display.shown.includes("labels")}
		teamId={issue.teamId}
		place="row"
		{always}
		subject={issue.reference}
		disabled={draftIDs.has(issue.id)}
		onchange={(labelIds, picked) => changeLabels(issue, labelIds, picked)}
	/>
{/snippet}

{#snippet assigneeControl(issue: Issue)}
	{@const held = names.get(issue.assigneeAccountId ?? "") ?? ""}
	<PropertyPicker
		options={[
			...people.map((member) => ({
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
				disabled={draftIDs.has(issue.id)}
				class="inline-flex size-6 cursor-pointer items-center justify-center rounded-sm hover:bg-paper-2"
			>
				{#if held}
					<PersonAvatar
						accountId={issue.assigneeAccountId ?? ""}
						name={held}
						size="xs"
						title={held}
					/>
				{:else}
					<Avatar.Root size="xs" variant="ghost" title="Unassigned">
						<Avatar.Fallback>+</Avatar.Fallback>
					</Avatar.Root>
				{/if}
			</button>
		{/snippet}
		{#snippet mark(option)}
			{#if option.value}
				<PersonAvatar accountId={option.value} name={option.label} size="xs" />
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
				{:else if team}
					<span class="flex min-w-0 shrink items-center gap-1.5">
						<TeamKey key={team.key} />
						<span class="truncate text-md text-muted-foreground">{team.name}</span>
					</span>
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
								class="relative inline-flex h-7.5 items-center gap-1.5 text-md font-medium whitespace-nowrap text-muted-foreground motion-control after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-transparent after:motion-control hover:text-ink-900 hover:after:bg-line-strong data-[active=true]:text-ink-900 data-[active=true]:after:bg-primary"
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
				<Button size="sm" disabled={teams.length === 0} onclick={() => raising.raise()}>
					<Plus aria-hidden="true" />
					New issue
				</Button>
			</div>
		</div>

		<div
			class="flex min-h-8.5 flex-wrap items-center gap-x-3 gap-y-1 border-t border-line-subtle py-1 pr-3 pl-3.5"
		>
			<FilterBar
				{facets}
				offered={pickableFacets}
				{catalogue}
				{linkWith}
				bind:open={filterOpen}
			/>

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
			<DisplayMenu
				{display}
				defaults={surfaceDefaults.issues}
				groupings={boardGroupings}
				orderings={surfaceOrderings.issues}
				{linkWith}
			/>
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

		{#if viewFailure}
			<div class="px-4 pt-3">
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not save that view</Alert.Title>
					<Alert.Description>{viewFailure}</Alert.Description>
				</Alert.Root>
			</div>
		{/if}

		{#if board.kind === "unavailable"}
			<div class="my-auto flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">We could not load these issues</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Nothing changed. Wait a moment and try again.
				</p>
				<Retry />
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
				<Button size="sm" disabled={teams.length === 0} onclick={() => raising.raise()}>
					<Plus aria-hidden="true" />
					New issue
				</Button>
			</div>
		{:else if data.layout === "list"}
			<div class="flex-1 overflow-auto">
				{#each columns as column (column.key)}
					{@const shut = collapsed.has(column.key)}
					<section
						aria-label={columnName(column)}
						ondragover={(event) => onDragOver(event, column.key)}
						ondragleave={(event) => onDragLeave(event, column.key)}
						ondrop={(event) => void onDrop(event, column.key)}
						data-dropping={dropTarget?.key === column.key}
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
									class="size-3.25 text-muted-foreground motion-control {shut
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
								aria-label="New issue in {columnName(column)}"
								disabled={teams.length === 0}
								onclick={() => raising.raise(seedFor(column.key))}
							>
								<Plus aria-hidden="true" />
							</Button>
						</div>
						{#if !shut}
							<div role="list">
								{#each column.issues as issue, index (issue.id)}
									{@render dropGap(column.key, index)}
									<IssueRow
										{issue}
										href={issueHref(issue.reference)}
										assignee={names.get(issue.assigneeAccountId ?? "") ?? ""}
										now={data.now}
										timezone={data.workspace.timezone}
										shown={display.shown}
										cursor={cursor.holds(issue)}
										selected={selected.has(issue.id)}
										pending={draftIDs.has(issue.id)}
										onselect={(extend) => toggle(issue.id, extend)}
										{priorityControl}
										{stateControl}
										{labelsControl}
										{assigneeControl}
										draggable
										dragging={dragging === issue.id}
										ondragstart={(event) => onDragStart(event, issue.id)}
										ondragend={onDragEnd}
									/>
								{/each}
								{@render dropGap(column.key, column.issues.length)}
							</div>
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
				<div class="flex min-h-full items-stretch gap-3">
					{#each columns as column (column.key)}
						<div
							role="group"
							aria-label={columnName(column)}
							ondragover={(event) => onDragOver(event, column.key)}
							ondragleave={(event) => onDragLeave(event, column.key)}
							ondrop={(event) => void onDrop(event, column.key)}
							data-dropping={dropTarget?.key === column.key}
							class="group/column flex w-60 flex-none sm:w-62.5 flex-col gap-2 rounded-lg border border-transparent p-1 motion-control data-[dropping=true]:border-dashed data-[dropping=true]:border-ink-400 data-[dropping=true]:bg-accent"
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
									aria-label="New issue in {columnName(column)}"
									disabled={teams.length === 0}
									onclick={() => raising.raise(seedFor(column.key))}
								>
									<Plus aria-hidden="true" />
								</Button>
							</div>

							<div role="list" class="flex flex-col gap-2">
								{#each column.issues as issue, index (issue.id)}
									{@render dropGap(column.key, index)}
									<IssueCard
										{issue}
										cursor={cursor.holds(issue)}
										href={issueHref(issue.reference)}
										assignee={names.get(issue.assigneeAccountId ?? "") ?? ""}
										now={data.now}
										timezone={data.workspace.timezone}
										shown={display.shown}
										selected={selected.has(issue.id)}
										pending={draftIDs.has(issue.id)}
										onselect={(extend) => toggle(issue.id, extend)}
										{priorityControl}
										{stateControl}
										{labelsControl}
										{assigneeControl}
										draggable
										dragging={dragging === issue.id}
										ondragstart={(event) => onDragStart(event, issue.id)}
										ondragend={onDragEnd}
									/>
								{/each}
								{@render dropGap(column.key, column.issues.length)}
							</div>

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
								disabled={teams.length === 0}
								onclick={() => raising.raise(seedFor(column.key))}
								class="flex h-7.5 w-full cursor-pointer items-center gap-1.75 rounded-md border border-dashed border-line-strong px-2 text-sm text-ink-600 opacity-0 motion-control group-hover/column:opacity-100 focus-visible:opacity-100 hover:bg-accent hover:text-foreground disabled:pointer-events-none"
							>
								<Plus class="size-3.5" aria-hidden="true" />
								New issue
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	{#if bulk}
		<div class="flex-none border-t border-line-default px-4 py-2">
			<BulkResult
				result={bulk}
				unreadable={bulkWatch.state.kind === "unreadable"}
				working={applying}
				onretry={() => bulkWatch.retry()}
				onretryfailed={retryFailed}
			/>
		</div>
	{/if}

	{#if selected.size > 0}
		<BulkBar
			bind:this={bulkBar}
			count={selected.size}
			states={sharedStates}
			members={people}
			cycles={sharedCycles}
			working={applying}
			onpriority={(priority) => applyBulk({ priority })}
			onstate={(stateId) => applyBulk({ stateId })}
			onreloadcycles={() => selectedTeam && loadCycles(selectedTeam)}
			oncycle={(cycleId) => applyBulk(cycleId === "" ? { clearCycle: true } : { cycleId })}
			onassignee={(accountId) =>
				applyBulk(accountId === "" ? { clearAssignee: true } : { assigneeId: accountId })}
			onstatus={(status) => applyBulk({ status })}
			onclear={clearSelection}
		/>
	{:else}
		<ShortcutBar
			ids={[
				"cursor-down",
				...(data.layout === "board" ? (["cursor-left", "cursor-right"] as const) : []),
				"cursor-open",
				"status-set",
				"select-toggle",
				...(cycling.size > 0 ? (["bulk-cycle"] as const) : []),
				"issue-filter",
				"issue-new",
				"help",
			]}
		>
			{#snippet lead()}
				{#if board.kind !== "unavailable"}
					<span class="font-mono text-xs text-muted-foreground tabular-nums">
						{total}
						{total === 1 ? "issue" : "issues"}
					</span>
				{/if}
			{/snippet}
		</ShortcutBar>
	{/if}
</div>

