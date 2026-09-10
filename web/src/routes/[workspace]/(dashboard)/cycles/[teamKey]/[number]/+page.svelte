<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Info from "@lucide/svelte/icons/info";
	import Layers from "@lucide/svelte/icons/layers";
	import Plus from "@lucide/svelte/icons/plus";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import { bindShortcuts, holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import { nthState, setStatus, statusIndexOf, statusMessage } from "$lib/issues/set-status";
	import { showToast } from "$lib/toast/toasts";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { registerNewIssue, useNewIssue } from "$lib/issues/new-issue.svelte";
	import { memberName } from "$lib/workspace/members";
	import { membersOf, rosterFor } from "$lib/team/members";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import {
		burndownCeiling,
		burndownIdeal,
		burndownRuns,
		burndownStart,
		issueAsRecorded,
		scopeAddedWithin,
		categoryOf,
		cycleCounts,
		cycleFailureMessage,
		cycleGroups,
		cyclePath,
		cycleResults,
		cycleRisks,
		cycleStanding,
		risksBecause,
		destinationLabels,
		plannedDays,
		readCycleFailure,
		rolloverLabels,
		scopeChangesOf,
		standingLabels,
		stateNameOf,
		teamCyclesPath,
		unfinishedIssues,
		unknownDays,
		unrecordedDays,
		type CycleFailure,
		type CycleRiskKind,
		type CycleRollover,
	} from "$lib/cycles/cycles";
	import { closeCycleSchema } from "$lib/cycles/close-schema";
	import { calendarDate, cycleWindow, daysBetween, onCalendarDate, onDate } from "$lib/time";
	import { teamSettingsPath } from "$lib/team/teams";
	import { workspacePath } from "$lib/workspace/navigation";
	import { cyclePreviewStates } from "./preview";
	import type { PageProps } from "./$types";
	import Retry from "$lib/components/norn/retry.svelte";
	import { attempt } from "$lib/api/attempt";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const zone = $derived(data.workspace.timezone);
	const preview = $derived(
		import.meta.env.DEV ? cyclePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const now = $derived(preview?.now ?? data.now);
	const today = $derived(calendarDate(now, zone));
	const detail = $derived(preview?.detail ?? data.detail);
	const progress = $derived(preview?.progress ?? data.progress);
	const states = $derived(preview?.states ?? data.states);
	const teamRoster = $derived(rosterFor(preview?.teamMembers ?? data.teamMembers));
	const roster = $derived(membersOf(teamRoster));
	const names = $derived(
		new Map(
			[
				...data.members.map((member) => [member.accountId, memberName(member)] as const),
				...roster.map((member) => [member.accountId, member.displayName] as const),
			].filter(([, name]) => name)
		)
	);

	const ready = $derived(detail.kind === "ready" ? detail : undefined);
	const cycle = $derived(ready?.cycle);
	const scope = $derived(ready?.scope);
	const standing = $derived(cycle ? cycleStanding(cycle, today) : undefined);
	const readOnly = $derived(standing?.kind === "closed");

	const results = $derived(ready ? cycleResults(ready.report) : undefined);
	const counts = $derived(results ? cycleCounts(results) : undefined);
	const groups = $derived(results ? cycleGroups(results) : []);
	const risks = $derived(results ? cycleRisks(results) : []);
	const unfinished = $derived(results ? unfinishedIssues(results) : []);
	const burndown = $derived(ready?.report.burndown);

	const addedIds = $derived(new Set((scope?.added ?? []).map((issue) => issue.id)));
	const joined = $derived(scope ? scopeChangesOf(scope, "added") : []);
	const removed = $derived(scope ? scopeChangesOf(scope, "removed") : []);
	const movedOn = $derived(
		scope ? [...scopeChangesOf(scope, "rolled_over"), ...scopeChangesOf(scope, "returned")] : []
	);

	const previousDone = $derived(
		ready?.previous?.frozen
			? ready.previous.results.filter((result) => result.category === "complete").length
			: null
	);
	const carriedIn = $derived(
		ready?.previous?.frozen
			? ready.previous.results.filter((result) => result.decision === "next").length
			: null
	);

	const rows = $derived(groups.flatMap((group) => group.issues));

	const cursor = listCursor(() => ({
		rows,
		open: (issue) => void goto(workspacePath(slug, `/issues/${issue.reference}`)),
	}));

	const raising = useNewIssue();

	registerNewIssue(() => ({
		seed: cycle && !readOnly ? { teamId: cycle.teamId, cycleId: cycle.id } : {},
	}));

	const canClose = $derived(cycle?.phase === "ended");

	holdShortcuts(() => canClose);

	bindShortcuts({
		"status-set": (binding) => void moveStatus(statusIndexOf(binding)),
	});

	async function moveStatus(nth: number) {
		if (readOnly) return;

		const issue = cursor.row;
		const state = issue && nthState(states ?? [], issue.teamId, nth);

		if (!issue || !state) return;

		const outcome = await setStatus(data.workspace.id, issue, state);

		if (outcome.kind !== "unchanged") {
			showToast(statusMessage(outcome, issue.reference), {
				href: workspacePath(slug, `/issues/${issue.reference}`),
			});
		}

		if (outcome.kind === "changed") {
			await Promise.all([
				invalidate(keys.page(page.route.id)),
				invalidate(keys.issues(data.workspace.id)),
			]);
		}
	}

	let failure = $state<CycleFailure | null>(null);
	let owning = $state(false);

	const hasNext = $derived(ready !== undefined && ready.nextNumber !== null);
	const rollovers: CycleRollover[] = ["next", "backlog", "keep"];
	const nextLabel = $derived(
		ready && ready.nextNumber !== null ? `Cycle ${ready.nextNumber}` : "the next cycle"
	);

	const closeForm = superForm(defaults(zod4(closeCycleSchema)), {
		id: "close-cycle",
		SPA: true,
		dataType: "json",
		validators: zod4Client(closeCycleSchema),
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !ready) return;

			failure = null;

			const reviewed = entered.data.reviewedIssueIds;

			const closing = ready.cycle.id;

			const outcome = await attempt({
				run: () =>
					api.POST("/workspaces/{workspaceId}/cycles/{cycleId}/close", {
						params: { path: { workspaceId: data.workspace.id, cycleId: closing } },
						body: {
							rollover: reviewed.length > 0 ? entered.data.decisions[reviewed[0]] : undefined,
							overrides: reviewed.map((issueId) => ({
								issueId,
								destination: entered.data.decisions[issueId],
							})),
							reviewedIssueIds: reviewed,
						},
					}),
			});

			if (outcome.kind === "refused") {
				await settle(readCycleFailure(outcome.problem), closing);

				return;
			}

			if (outcome.kind === "unknown") {
				await settle({ kind: "unavailable" }, closing);

				return;
			}

			await invalidate(keys.page(page.route.id));
		},
	});

	const { form: decided, enhance: closeEnhance, submitting: closing_ } = closeForm;

	const working = $derived($closing_);
	const settled = $derived(unfinished.filter((issue) => $decided.decisions[issue.id]).length);
	const undecided = $derived(settled < unfinished.length);

	let formedFor = $state("");

	$effect(() => {
		const at = cycle?.id ?? "";
		const reviewing = unfinished.map((issue) => issue.id);

		if (at === "") return;

		if (at !== formedFor) {
			formedFor = at;
			failure = null;
		}

		decided.set({ reviewedIssueIds: reviewing, decisions: {} });
	});

	const chart = $derived.by(() => {
		if (!cycle || !burndown) return undefined;

		const days = plannedDays(cycle);
		const ideal = burndownIdeal(cycle, burndown);
		const highest = Math.max(burndownCeiling(burndown), burndownStart(burndown) ?? 0);

		if (days === 0 || burndown.points.length === 0) return undefined;

		const ceiling = Math.max(highest, 1);
		const x = (index: number) => (days > 1 ? (index / (days - 1)) * 600 : 300);
		const y = (value: number) => 139 - (value / ceiling) * 133;
		const path = (values: { at: number; value: number }[]) =>
			values.map((point, at) => `${at === 0 ? "M" : "L"}${x(point.at)} ${y(point.value)}`).join(" ");

		const runs = burndownRuns(burndown);

		return {
			ideal: ideal.length > 0 ? path(ideal.map((value, at) => ({ at, value }))) : "",
			runs: runs
				.filter((run) => run.points.length > 1)
				.map((run) =>
					path(run.points.map((point, at) => ({ at: run.from + at, value: point.remaining })))
				),
			dots: runs
				.filter((run) => run.points.length === 1)
				.map((run) => ({ x: x(run.from), y: y(run.points[0].remaining) })),
			ticks:
				cycle.startsOn === cycle.endsOn
					? [{ at: "start", label: onCalendarDate(cycle.startsOn) }]
					: [
							{ at: "start", label: onCalendarDate(cycle.startsOn) },
							{ at: "end", label: onCalendarDate(cycle.endsOn) },
						],
			missing: unrecordedDays(burndown),
			uncertain: unknownDays(burndown),
		};
	});

	const stats = $derived.by(() => {
		if (!cycle || !counts || !standing) return [];

		if (standing.kind === "upcoming") {
			return [
				{ key: "planned", value: `${counts.scope}`, sub: "issues" },
				{
					key: "starts",
					value: onCalendarDate(cycle.startsOn),
					sub:
						daysBetween(today, cycle.startsOn) === 1
							? "tomorrow"
							: `in ${daysBetween(today, cycle.startsOn)} days`,
				},
				{
					key: "capacity",
					value: previousDone === null ? "—" : `${previousDone}`,
					sub: previousDone === null ? "no recorded cycle before this" : "last cycle",
				},
				{
					key: "carried over",
					value: carriedIn === null ? "—" : `${carriedIn}`,
					sub: carriedIn === null ? "no recorded cycle before this" : "from the cycle before",
				},
				{
					key: "team",
					value: teamRoster.kind === "unavailable" ? "—" : `${roster.length}`,
					sub:
						teamRoster.kind === "unavailable"
							? "we could not read the team"
							: roster.length === 1
								? "person"
								: "people",
				},
			];
		}

		return [
			{ key: "scope", value: `${counts.scope}`, sub: "issues" },
			{
				key: "done",
				value: `${counts.done}`,
				sub: counts.scope ? `${Math.round((counts.done / counts.scope) * 100)}%` : "0%",
			},
			{ key: "in flight", value: `${counts.inFlight}`, sub: "started" },
			{ key: "not started", value: `${counts.notStarted}`, sub: "" },
			{
				key: "at risk",
				value: `${risks.length}`,
				sub: risks.length ? "need a nudge" : "clear",
			},
		];
	});

	const facts = $derived.by(() => {
		if (!cycle || !counts || !standing) return [];

		if (standing.kind === "upcoming") {
			return [
				{ key: "starts", value: onCalendarDate(cycle.startsOn) },
				{ key: "ends", value: onCalendarDate(cycle.endsOn) },
				{ key: "team", value: cycle.teamKey },
				{ key: "planned", value: `${counts.scope} issues` },
				{ key: "carry-over", value: carriedIn === null ? "—" : `${carriedIn}` },
			];
		}

		return [
			{ key: "start", value: onCalendarDate(cycle.startsOn) },
			{ key: "end", value: onCalendarDate(cycle.endsOn) },
			{ key: "team", value: cycle.teamKey },
			{ key: "scope", value: `${counts.scope} issues` },
			{ key: "added mid-cycle", value: `${scope ? scopeAddedWithin(scope, cycle) : 0}` },
			{ key: "carried in", value: carriedIn === null ? "—" : `${carriedIn}` },
		];
	});

	const alert = $derived.by(() => {
		if (!cycle || !standing) return undefined;

		if (standing.kind === "ending") {
			const left =
				standing.daysLeft === 0
					? "Last day"
					: `${standing.daysLeft} ${standing.daysLeft === 1 ? "day" : "days"} left`;

			if (risks.length === 0) {
				return {
					tone: "info" as const,
					title: `${cycle.name} ends soon`,
					text: `${left}. Nothing needs a nudge. Review unfinished work when you close the cycle.`,
				};
			}

			const counted: [CycleRiskKind, string][] = [
				["blocked", "blocked"],
				["unassigned", "with nobody assigned"],
				["stale", "with no recent status change"],
			];

			const because = counted
				.map(([kind, label]) => [risksBecause(risks, kind), label] as const)
				.filter(([count]) => count > 0)
				.map(([count, label]) => `${count} ${label}`);

			return {
				tone: "warning" as const,
				title: `${risks.length} ${risks.length === 1 ? "issue needs" : "issues need"} a nudge`,
				text: `${left}. ${because.join(", ")}. Move them to ${nextLabel} now, or accept the roll-over.`,
			};
		}

		if (standing.kind === "closed") {
			return {
				tone: "info" as const,
				title: `${cycle.name} is read only`,
				text:
					results?.kind === "final" && cycle.closedAt
						? `Nothing here can be edited. Numbers are final as of ${onDate(cycle.closedAt, zone)}.`
						: "Nothing here can be edited.",
			};
		}

		if (standing.kind === "upcoming") {
			return {
				tone: "info" as const,
				title: "Scope can still change",
				text:
					`${cycle.name} starts ${onCalendarDate(cycle.startsOn)}. ` +
					"Issues added before then count as planned scope, not carry-over.",
			};
		}

		return undefined;
	});

	async function settle(read: CycleFailure, cycleId: string) {
		failure = read;

		if (read.kind === "stale") {
			await invalidate(keys.page(page.route.id));

			return;
		}

		if (read.kind !== "closed" && read.kind !== "unavailable") return;

		const standing = await closedNow(cycleId);

		if (standing === "closed") {
			failure = null;
			await invalidate(keys.page(page.route.id));

			return;
		}

		if (standing === "unreadable") failure = { kind: "uncertain" };
	}

	async function closedNow(cycleId: string): Promise<"closed" | "open" | "unreadable"> {
		try {
			const report = await api.GET("/workspaces/{workspaceId}/cycles/{cycleId}/report", {
				params: { path: { workspaceId: data.workspace.id, cycleId } },
			});

			if (report.error || !report.data) return "unreadable";

			return report.data.phase === "closed" ? "closed" : "open";
		} catch {
			return "unreadable";
		}
	}

	function decideAll(destination: CycleRollover) {
		decided.update((entered) => ({
			...entered,
			decisions: Object.fromEntries(unfinished.map((issue) => [issue.id, destination])),
		}));
	}

	async function setOwner(accountId: string) {
		if (!ready) return;

		owning = true;
		failure = null;

		const outcome = await attempt({
			run: () =>
				api.PUT("/workspaces/{workspaceId}/cycles/{cycleId}/owner", {
					params: { path: { workspaceId: data.workspace.id, cycleId: ready.cycle.id } },
					body: { ownerAccountId: accountId || null },
				}),
		});

		owning = false;

		if (outcome.kind === "refused") {
			failure = readCycleFailure(outcome.problem);

			return;
		}

		if (outcome.kind === "unknown") {
			failure = { kind: "uncertain" };

			return;
		}

		await invalidate(keys.page(page.route.id));
	}

	const ownerName = $derived.by(() => {
		const owner = cycle?.ownerAccountId;

		if (!owner) return "Unassigned";

		return names.get(owner) ?? "Name not available";
	});
</script>

<svelte:head>
	<title>
		{cycle ? cycle.name : "Cycle"} · {data.workspace.name} · Norn
	</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex min-h-11 flex-wrap items-center gap-2 py-1.5 pr-3 pl-4">
			<Layers class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={teamCyclesPath(slug, page.params.teamKey ?? "")}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Cycles
			</a>
			{#if cycle && standing}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
					{cycle.name}
				</h1>
				<span
					class="shrink-0 rounded-full bg-paper-2 px-2 py-0.5 text-xs text-ink-700"
					data-standing={standing.kind}
				>
					{standingLabels[standing.kind]}
				</span>
				<span class="hidden h-3.5 w-px bg-line-default sm:block" aria-hidden="true"></span>
				<span class="hidden font-mono text-xs text-muted-foreground tabular-nums sm:inline">
					{cycleWindow(cycle.startsOn, cycle.endsOn)}
				</span>
				<span class="text-xs text-muted-foreground">
					{#if standing.kind === "running"}
						day {standing.day} of {standing.of}
					{:else if standing.kind === "ending"}
						{#if standing.daysLeft === 0}
							last day
						{:else}
							{standing.daysLeft}
							{standing.daysLeft === 1 ? "day" : "days"} left
						{/if}
					{:else if standing.kind === "upcoming"}
						{#if daysBetween(today, cycle.startsOn) === 1}
							starts tomorrow
						{:else}
							starts in {daysBetween(today, cycle.startsOn)} days
						{/if}
					{:else if standing.kind === "ended"}
						ended {onCalendarDate(cycle.endsOn)}
					{:else}
						closed {cycle.closedAt ? onDate(cycle.closedAt, zone) : ""}
					{/if}
				</span>

				<div class="ml-auto flex items-center gap-2">
					{#if progress && results?.kind === "live"}
						<ProgressBar {progress} class="hidden lg:inline-flex" />
					{/if}
					{#if ready}
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="ghost" size="sm">
										Switch cycle
										<ChevronDown class="size-3.5" aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end">
								<DropdownMenu.Group>
									<DropdownMenu.GroupHeading>Cycles</DropdownMenu.GroupHeading>
									{#each ready.others as other (other.id)}
										<DropdownMenu.Item>
											{#snippet child({ props })}
												<a {...props} href={cyclePath(slug, other)}>
													{other.name}
													<span class="ml-auto text-xs text-muted-foreground">
														{standingLabels[cycleStanding(other, today).kind]}
													</span>
												</a>
											{/snippet}
										</DropdownMenu.Item>
									{/each}
								</DropdownMenu.Group>
								<DropdownMenu.Separator />
								<DropdownMenu.Item>
									{#snippet child({ props })}
										<a {...props} href={teamSettingsPath(slug, cycle.teamKey)}>Cycle settings</a>
									{/snippet}
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					{/if}
					<Button
						size="sm"
						disabled={readOnly || !cycle}
						onclick={() => raising.raise()}
						title={readOnly ? "This cycle is closed" : undefined}
					>
						<Plus class="size-3.5" aria-hidden="true" />
						New issue
					</Button>
				</div>
			{/if}
		</div>

		{#if stats.length > 0}
			{#if results?.kind === "unrecorded"}
				<p class="border-t border-line-default px-4 pt-2 text-xs text-muted-foreground">
					These are today's numbers. What {cycle?.name} held when it closed was not recorded.
				</p>
			{/if}
			<div
				class="flex flex-wrap gap-x-6 gap-y-2 px-4 py-2 {results?.kind === 'unrecorded'
					? ''
					: 'border-t border-line-default'}"
			>
				{#each stats as stat (stat.key)}
					<div class="flex flex-col gap-0.5">
						<span class="text-xs text-muted-foreground">{stat.key}</span>
						<span class="flex items-baseline gap-1.5">
							<span class="font-mono text-md text-ink-900 tabular-nums">{stat.value}</span>
							{#if stat.sub}
								<span class="text-xs text-muted-foreground">{stat.sub}</span>
							{/if}
						</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-260 flex-col gap-4 px-4 py-4 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))] lg:flex-row lg:items-start"
		>
			{#if detail.kind === "loading"}
				<div class="h-40 flex-1 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if detail.kind === "not_found"}
				<div class="flex flex-1 flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">No cycle here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no cycle at this address, or it belongs to a team you cannot see.
					</p>
					<div>
						<Button
							variant="secondary"
							size="sm"
							href={teamCyclesPath(slug, page.params.teamKey ?? "")}
						>
							All cycles
						</Button>
					</div>
				</div>
			{:else if detail.kind === "unavailable"}
				<div class="flex flex-1 flex-col items-start gap-3">
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>We could not load this cycle</Alert.Title>
						<Alert.Description>Wait a moment and try again.</Alert.Description>
					</Alert.Root>
					<Retry />
				</div>
			{:else if ready && cycle && counts && results}
				<div class="flex min-w-0 flex-1 flex-col gap-4">
					{#if failure}
						<Alert.Root variant="destructive">
							<TriangleAlert aria-hidden="true" />
							<Alert.Title>
								{failure.kind === "uncertain"
									? "We could not confirm the result"
									: "That did not go through"}
							</Alert.Title>
							<Alert.Description>{cycleFailureMessage(failure)}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if alert}
						<Alert.Root variant={alert.tone === "warning" ? "warning" : "default"}>
							{#if alert.tone === "warning"}
								<TriangleAlert aria-hidden="true" />
							{:else}
								<Info aria-hidden="true" />
							{/if}
							<Alert.Title>{alert.title}</Alert.Title>
							<Alert.Description>{alert.text}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if results.kind === "unrecorded"}
						<Alert.Root>
							<Info aria-hidden="true" />
							<Alert.Title>What each issue looked like here was not recorded</Alert.Title>
							<Alert.Description>
								{cycle.name} closed before Norn kept per-issue results, so its final numbers cannot
								be shown. The issues below are how they stand today, not how they stood when it
								closed.
							</Alert.Description>
						</Alert.Root>
					{/if}

					{#if canClose}
							<form
							method="POST"
							use:closeEnhance
							class="flex flex-col gap-3 rounded-lg border border-line-strong bg-paper-1 p-3"
							aria-labelledby="closing-heading"
						>
							<div class="flex flex-wrap items-center gap-2">
								<h2 id="closing-heading" class="text-sm text-ink-900">
									Unfinished · decide before closing
								</h2>
								{#if unfinished.length > 0}
									<Button
										type="button"
										variant="ghost"
										size="sm"
										class="ml-auto"
										disabled={working || !hasNext}
										title={hasNext ? undefined : "There is no later cycle to move these into"}
										onclick={() => decideAll("next")}
									>
										Move all to {nextLabel}
									</Button>
								{/if}
							</div>

							{#if unfinished.length === 0}
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									Nothing is unfinished, so there is nothing to move.
								</p>
							{:else}
								<ul class="flex flex-col gap-1.5">
									{#each unfinished as issue (issue.id)}
										<li class="flex flex-wrap items-center justify-between gap-2">
											<span class="flex min-w-0 items-center gap-2">
												<StatusIcon
													category={categoryOf(results, issue)}
													name={stateNameOf(results, issue)}
													decorative
												/>
												<span class="font-mono text-xs text-muted-foreground tabular-nums">
													{issue.reference}
												</span>
												<span class="min-w-0 truncate text-sm text-ink-900">{issue.title}</span>
											</span>
											<RadioGroup.Root
												bind:value={
													() => $decided.decisions[issue.id] ?? "",
													(value) =>
														decided.update((entered) => ({
															...entered,
															decisions: {
																...entered.decisions,
																[issue.id]: value as CycleRollover,
															},
														}))
												}
												disabled={working}
												orientation="horizontal"
												aria-label="Where {issue.reference} goes"
												class="flex w-auto shrink-0 gap-1"
											>
												{#each rollovers as destination (destination)}
													<RadioGroup.Item
														variant="chip"
														value={destination}
														disabled={destination === "next" && !hasNext}
													>
														{destinationLabels(nextLabel)[destination]}
													</RadioGroup.Item>
												{/each}
											</RadioGroup.Root>
										</li>
									{/each}
								</ul>
							{/if}

							<div class="flex flex-wrap items-center gap-2">
								<span class="text-xs text-muted-foreground" aria-live="polite">
									{settled} of {unfinished.length} decided
								</span>
								<div class="ml-auto flex gap-2">
									<Button type="submit" disabled={working || undecided}>
										{working ? "Closing" : `Close ${cycle.name}`}
									</Button>
								</div>
							</div>
						</form>
					{/if}

					<section
						class="flex flex-col gap-2 rounded-lg border border-line-default bg-paper-1 p-3"
						aria-labelledby="burndown-heading"
					>
						<div class="flex flex-wrap items-baseline gap-2">
							<h2 id="burndown-heading" class="text-sm text-ink-900">Remaining work</h2>
							<span class="ml-auto flex items-center gap-3 text-xs text-muted-foreground">
								<span class="flex items-center gap-1.5">
									<span class="h-px w-3 bg-primary" aria-hidden="true"></span>actual
								</span>
								<span class="flex items-center gap-1.5">
									<span class="h-px w-3 bg-line-strong" aria-hidden="true"></span>ideal
								</span>
							</span>
						</div>

						{#if chart}
							<div>
								<svg
									viewBox="0 0 600 140"
									preserveAspectRatio="none"
									class="h-32 w-full"
									role="img"
									aria-label="Remaining work across {cycle.name}"
								>
									<line x1="0" y1="35" x2="600" y2="35" class="stroke-line-default" />
									<line x1="0" y1="70" x2="600" y2="70" class="stroke-line-default" />
									<line x1="0" y1="105" x2="600" y2="105" class="stroke-line-default" />
									<line x1="0" y1="139" x2="600" y2="139" class="stroke-line-strong" />
									{#if chart.ideal}
										<path
											d={chart.ideal}
											fill="none"
											vector-effect="non-scaling-stroke"
											class="stroke-line-strong"
										/>
									{/if}
									{#each chart.runs as run, at (at)}
										<path
											d={run}
											fill="none"
											vector-effect="non-scaling-stroke"
											stroke-width="2"
											class="stroke-primary"
										/>
									{/each}
									{#each chart.dots as dot, at (at)}
										<circle cx={dot.x} cy={dot.y} r="3" class="fill-primary" />
									{/each}
								</svg>
							</div>
							<div class="flex justify-between font-mono text-xs text-muted-foreground tabular-nums">
								{#each chart.ticks as tick (tick.at)}
									<span>{tick.label}</span>
								{/each}
							</div>
							{#if chart.missing > 0}
								<p class="text-xs leading-normal text-muted-foreground text-pretty">
									The first {chart.missing}
									{chart.missing === 1 ? "day is" : "days are"} not drawn. They are before Norn began
									recording what cycles contained.
								</p>
							{/if}
							{#if chart.uncertain > 0}
								<p class="text-xs leading-normal text-muted-foreground text-pretty">
									{chart.uncertain}
									{chart.uncertain === 1 ? "day is" : "days are"} not drawn because some issues' standing
									that day is not in the record.
								</p>
							{/if}
							{#if !chart.ideal}
								<p class="text-xs leading-normal text-muted-foreground text-pretty">
									The ideal line needs the scope this cycle started with, and that day is not in the
									record.
								</p>
							{/if}
						{:else}
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{#if standing?.kind === "upcoming"}
									Nothing is drawn until {cycle.name} starts.
								{:else if counts.scope === 0}
									Nothing is in {cycle.name}, so there is nothing to draw.
								{:else}
									There is no recorded history to draw for {cycle.name}.
								{/if}
							</p>
						{/if}

						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Scope</span>
							<span
								class="flex h-1.5 flex-1 overflow-hidden rounded-full bg-paper-3"
								role="img"
								aria-label="{counts.done} of {counts.scope} done"
							>
								{#if counts.scope > 0}
									<span
										class="bg-status-complete"
										style="width: {(counts.done / counts.scope) * 100}%"
									></span>
									<span
										class="bg-status-active"
										style="width: {(counts.inFlight / counts.scope) * 100}%"
									></span>
									<span
										class="bg-status-not-started"
										style="width: {(counts.notStarted / counts.scope) * 100}%"
									></span>
								{/if}
							</span>
							<span class="font-mono text-xs text-muted-foreground tabular-nums">
								{counts.done}/{counts.scope} done{results.kind === "unrecorded" ? " today" : ""}
							</span>
						</div>
					</section>

					{#if counts.scope === 0}
						<div class="flex flex-col gap-2 rounded-lg border border-line-default p-4">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								Nothing in {cycle.name} yet
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Add issues from the list, or start one here. Scope is easiest to judge on day one.
							</p>
							<div>
								<Button
									variant="secondary"
									size="sm"
									href={workspacePath(slug, "/issues")}
									disabled={readOnly}
								>
									Add issues from the list
								</Button>
							</div>
						</div>
					{:else}
						<section class="flex flex-col gap-2">
							{#each groups as group (group.category)}
								<div class="flex flex-col">
									<div class="flex items-center gap-2 py-1.5">
										<StatusIcon category={group.category} name={group.label} decorative />
										<span class="text-sm text-ink-900">{group.label}</span>
										<span class="font-mono text-xs text-muted-foreground tabular-nums">
											{group.issues.length}
										</span>
									</div>
									<ul class="flex flex-col">
										{#each group.issues as issue (issue.id)}
											<li class="flex items-center gap-2">
												<IssueRow
													issue={issueAsRecorded(results, issue)}
													assignee={names.get(issue.assigneeAccountId ?? "") ?? ""}
													href={workspacePath(slug, `/issues/${issue.reference}`)}
													now={now}
													timezone={zone}
													cursor={cursor.holds(issue)}
													class="min-w-0 flex-1"
												/>
												{#if addedIds.has(issue.id)}
													<span
														class="shrink-0 text-xs text-muted-foreground"
														title="Added after this cycle started"
													>
														added later
													</span>
												{/if}
											</li>
										{/each}
									</ul>
								</div>
							{/each}
						</section>
					{/if}

					{#if removed.length > 0}
						<section class="flex flex-col gap-2">
							<Eyebrow class="text-ink-600">Taken out after it started</Eyebrow>
							<ul class="flex flex-col gap-1">
								{#each removed as change (change.id)}
									<li class="flex min-w-0 items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground tabular-nums">
											{change.issueReference}
										</span>
										<a
											href={workspacePath(slug, `/issues/${change.issueReference}`)}
											class="min-w-0 truncate text-sm text-ink-900 motion-control hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
										>
											{change.issueTitle}
										</a>
									</li>
								{/each}
							</ul>
						</section>
					{/if}

					{#if movedOn.length > 0}
						<section class="flex flex-col gap-2">
							<Eyebrow class="text-ink-600">Moved on when it closed</Eyebrow>
							<ul class="flex flex-col gap-1">
								{#each movedOn as change (change.id)}
									<li class="flex min-w-0 items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground tabular-nums">
											{change.issueReference}
										</span>
										<a
											href={workspacePath(slug, `/issues/${change.issueReference}`)}
											class="min-w-0 truncate text-sm text-ink-900 motion-control hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
										>
											{change.issueTitle}
										</a>
										<span class="ml-auto shrink-0 text-xs text-muted-foreground">
											{change.change === "rolled_over" ? "Next cycle" : "Backlog"}
										</span>
									</li>
								{/each}
							</ul>
						</section>
					{/if}
				</div>

				<aside class="flex w-full shrink-0 flex-col gap-4 lg:w-70">
					{#if risks.length > 0}
						<section class="flex flex-col gap-2" aria-labelledby="risk-heading">
							<h2 id="risk-heading" class="text-xs text-muted-foreground">
								At risk · {risks.length}
							</h2>
							{#each risks as risk (risk.issue.id)}
								<a
									href={workspacePath(slug, `/issues/${risk.issue.reference}`)}
									class="flex flex-col gap-1 rounded-lg border border-line-default p-2 motion-control hover:border-line-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									<span class="flex items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground tabular-nums">
											{risk.issue.reference}
										</span>
										<StatusIcon
											category={categoryOf(results, risk.issue)}
											name={stateNameOf(results, risk.issue)}
											decorative
										/>
										{#if names.get(risk.issue.assigneeAccountId ?? "")}
											<Avatar.Root class="ml-auto size-4.5">
												<Avatar.Fallback class="text-2xs">
													{names.get(risk.issue.assigneeAccountId ?? "")?.slice(0, 1)}
												</Avatar.Fallback>
											</Avatar.Root>
										{/if}
									</span>
									<span class="truncate text-sm text-ink-900">{risk.issue.title}</span>
									{#each risk.reasons as reason (reason.kind)}
										<span class="flex items-center gap-1.5 text-xs text-warning">
											<TriangleAlert class="size-3" aria-hidden="true" />
											{reason.text}
										</span>
									{/each}
								</a>
							{/each}
						</section>
					{/if}

					<section
						class="flex flex-col gap-2 rounded-lg border border-line-default p-3"
						aria-labelledby="facts-heading"
					>
						<h2 id="facts-heading" class="text-xs text-muted-foreground">Cycle</h2>
						<dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
							{#each facts as fact (fact.key)}
								<dt class="text-xs text-muted-foreground">{fact.key}</dt>
								<dd class="text-right text-xs text-ink-900">{fact.value}</dd>
							{/each}
						</dl>
						<div class="flex flex-col gap-1.5">
							<span class="text-xs text-muted-foreground" id="owner-label">owner</span>
							{#if readOnly}
								<span class="text-xs text-ink-900">{ownerName}</span>
							{:else if teamRoster.kind === "unavailable"}
								<span class="text-xs text-ink-900">{ownerName}</span>
								<span class="text-xs leading-normal text-muted-foreground text-pretty">
									We could not read who is on this team, so the owner cannot be changed here right
									now.
								</span>
							{:else}
								<Select.Root
									type="single"
									value={cycle.ownerAccountId ?? ""}
									onValueChange={(value) => void setOwner(value)}
									disabled={owning}
								>
									<Select.Trigger class="w-full" aria-labelledby="owner-label">
										{ownerName}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="">Unassigned</Select.Item>
										{#each roster as member (member.accountId)}
											<Select.Item value={member.accountId}>{member.displayName}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							{/if}
						</div>
					</section>

					{#if ready.others.length > 0}
						<section class="flex flex-col gap-1" aria-labelledby="others-heading">
							<h2 id="others-heading" class="text-xs text-muted-foreground">Other cycles</h2>
							{#each ready.others as other (other.id)}
								<a
									href={cyclePath(slug, other)}
									class="flex items-center gap-2 rounded-md px-2 py-1.5 motion-row hover:bg-paper-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									<span class="text-sm text-ink-900">{other.name}</span>
									<span class="ml-auto text-xs text-muted-foreground">
										{standingLabels[cycleStanding(other, today).kind]}
									</span>
								</a>
							{/each}
						</section>
					{/if}
				</aside>
			{/if}
		</div>
	</div>

	<ShortcutBar
		ids={readOnly
			? ["cursor-down", "cursor-open", "help"]
			: ["cursor-down", "cursor-open", "status-set", "issue-new", "help"]}
	/>
</div>
