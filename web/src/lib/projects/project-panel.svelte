<script lang="ts">
	import Calendar from "@lucide/svelte/icons/calendar";
	import Clock from "@lucide/svelte/icons/clock";
	import Folder from "@lucide/svelte/icons/folder";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import { categoryLabels, type StateCategory } from "$lib/team/states";
	import { totalIssues, type IssueProgress } from "$lib/issues/board";
	import { onCalendarDate, onDate } from "$lib/time";
	import type { Team } from "$lib/team/teams";
	import { projectStanding, teamsLine, type Project, type ProjectMember } from "./projects";

	let {
		project,
		members,
		progress,
		teams,
		timezone,
	}: {
		project: Project;
		members: ProjectMember[];
		progress: IssueProgress | undefined;
		teams: Team[];
		timezone: string;
	} = $props();

	const breakdown = $derived<{ category: StateCategory; count: number }[]>(
		progress
			? (
					[
						{ category: "complete", count: progress.complete },
						{ category: "active", count: progress.active },
						{ category: "not_started", count: progress.notStarted },
						{ category: "abandoned", count: progress.abandoned },
					] as { category: StateCategory; count: number }[]
				).filter((row) => row.count > 0)
			: []
	);
</script>

<aside class="flex flex-col gap-6">
	<section class="flex flex-col gap-2.5">
		<Eyebrow class="text-ink-600">Properties</Eyebrow>
		<dl class="flex flex-col gap-2 text-sm">
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<UserRound class="size-icon-row" aria-hidden="true" />
					Lead
				</dt>
				<dd class="min-w-0 truncate text-ink-900">{project.leadName || "Nobody yet"}</dd>
			</div>
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<Users class="size-icon-row" aria-hidden="true" />
					Members
				</dt>
				<dd class="font-mono text-ink-900 tabular-nums">
					{members.length === 1 ? "1 person" : `${members.length} people`}
				</dd>
			</div>
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<Calendar class="size-icon-row" aria-hidden="true" />
					Target
				</dt>
				<dd
					class="font-mono tabular-nums"
					class:text-warning={projectStanding(project) === "at_risk"}
					class:text-ink-900={projectStanding(project) !== "at_risk"}
				>
					{project.targetOn ? onCalendarDate(project.targetOn) : "None"}
				</dd>
			</div>
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<Folder class="size-icon-row" aria-hidden="true" />
					Teams
				</dt>
				<dd class="min-w-0 truncate text-ink-900">{teamsLine(project, teams)}</dd>
			</div>
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<Clock class="size-icon-row" aria-hidden="true" />
					Started
				</dt>
				<dd class="font-mono text-ink-900 tabular-nums">
					{onDate(project.createdAt, timezone)}
				</dd>
			</div>
		</dl>
	</section>

	{#if progress && totalIssues(progress) > 0}
		<section class="flex flex-col gap-2.5">
			<Eyebrow class="text-ink-600">Progress</Eyebrow>
			<dl class="flex flex-col gap-2 text-sm">
				{#each breakdown as row (row.category)}
					<div class="flex items-baseline justify-between gap-3">
						<dt class="flex items-center gap-1.5 text-muted-foreground">
							<StatusIcon category={row.category} decorative class="size-icon-row" />
							{categoryLabels[row.category]}
						</dt>
						<dd class="font-mono text-ink-900 tabular-nums">{row.count}</dd>
					</div>
				{/each}
			</dl>
		</section>
	{/if}
</aside>
