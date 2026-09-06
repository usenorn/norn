<script lang="ts">
	import ArrowUpRight from "@lucide/svelte/icons/arrow-up-right";
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
	import { projectStanding, teamsLine, type Project, type ProjectLink, type ProjectMember } from "./projects";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Link2 from "@lucide/svelte/icons/link-2";
	import Plus from "@lucide/svelte/icons/plus";
	import X from "@lucide/svelte/icons/x";

	let {
		project,
		members,
		links,
		progress,
		teams,
		timezone,
		locked = false,
		onadd,
		onremove,
	}: {
		project: Project;
		members: ProjectMember[];
		links: ProjectLink[];
		progress: IssueProgress | undefined;
		teams: Team[];
		timezone: string;
		locked?: boolean;
		onadd: (label: string, url: string) => Promise<void>;
		onremove: (linkId: string) => Promise<void>;
	} = $props();

	let adding = $state(false);
	let label = $state("");
	let address = $state("");
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();

		if (busy || label.trim() === "" || address.trim() === "") return;

		busy = true;

		try {
			await onadd(label.trim(), address.trim());
			label = "";
			address = "";
			adding = false;
		} finally {
			busy = false;
		}
	}

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
			<div class="flex items-baseline justify-between gap-3">
				<dt class="flex items-center gap-1.5 text-muted-foreground">
					<ArrowUpRight class="size-icon-row" aria-hidden="true" />
					Raised
				</dt>
				<dd class="font-mono text-ink-900 tabular-nums">
					+{progress?.raisedSinceStart ?? 0} since start
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
	<section class="flex flex-col gap-2.5">
		<Eyebrow class="text-ink-600">Links</Eyebrow>
		{#if links.length > 0}
			<ul class="flex flex-col gap-1.5">
				{#each links as link (link.id)}
					<li class="group flex items-center gap-1.5">
						<Link2 class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
						<a
							href={link.url}
							target="_blank"
							rel="noreferrer noopener"
							class="min-w-0 flex-1 truncate text-sm text-link motion-control hover:underline"
						>
							{link.label}
						</a>
						{#if !locked}
							<Button
								variant="ghost"
								size="icon-sm"
								aria-label="Remove {link.label}"
								onclick={() => onremove(link.id)}
								class="opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
							>
								<X aria-hidden="true" />
							</Button>
						{/if}
					</li>
				{/each}
			</ul>
		{:else if locked}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Nothing pinned here yet.
			</p>
		{/if}

		{#if !locked}
			{#if adding}
				<form onsubmit={submit} class="flex flex-col gap-1.5">
					<Input bind:value={label} placeholder="What it is" disabled={busy} autocomplete="off" />
					<Input
						bind:value={address}
						type="url"
						placeholder="https://"
						disabled={busy}
						autocomplete="off"
					/>
					<div class="flex gap-1.5">
						<Button type="submit" size="sm" disabled={busy || !label.trim() || !address.trim()}>
							{busy ? "Pinning" : "Pin it"}
						</Button>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							disabled={busy}
							onclick={() => (adding = false)}
						>
							Cancel
						</Button>
					</div>
				</form>
			{:else}
				<div>
					<Button variant="ghost" size="sm" class="-ml-2" onclick={() => (adding = true)}>
						<Plus aria-hidden="true" />
						Pin a link
					</Button>
				</div>
			{/if}
		{/if}
	</section>
</aside>
