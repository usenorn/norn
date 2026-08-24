<script lang="ts">
	import { page } from "$app/state";
	import { goto } from "$app/navigation";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Layers from "@lucide/svelte/icons/layers";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { cyclePath, phaseLabel, type Cycle } from "$lib/cycles/cycles";
	import { cycleWindow } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamCyclesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV
			? teamCyclesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const listing = $derived(preview?.listing ?? data.listing);

	const grouped = $derived.by(() => {
		if (listing.kind !== "ready") return [];

		const sections: { label: string; cycles: Cycle[] }[] = [
			{ label: "In progress", cycles: listing.cycles.filter((c) => c.phase === "current") },
			{ label: "Needs closing", cycles: listing.cycles.filter((c) => c.phase === "ended") },
			{ label: "Upcoming", cycles: listing.cycles.filter((c) => c.phase === "upcoming") },
			{
				label: "Past",
				cycles: listing.cycles.filter((c) => c.phase === "closed").reverse(),
			},
		];

		return sections.filter((section) => section.cycles.length > 0);
	});

	const rows = $derived(grouped.flatMap((section) => section.cycles));

	const cursor = listCursor(() => ({
		rows,
		open: (cycle) => void goto(cyclePath(slug, cycle)),
	}));
</script>

<svelte:head>
	<title>Cycles · {data.workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Layers class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
				{listing.kind === "ready" ? `${listing.teamName} cycles` : "Cycles"}
			</h1>
		</div>
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if listing.kind === "loading"}
				<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if listing.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">No team here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no team at this address in {data.workspace.name}, or it is private and you are
						not on it.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={workspacePath(slug, "/settings/teams")}>
							Back to teams
						</Button>
					</div>
				</div>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load these cycles</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "disabled"}
				<div class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">
						This team does not use cycles
					</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Cycles are a repeating time box a team plans against. Turn them on and Norn keeps the
						next few ready.
					</p>
					<div>
						<Button
							variant="secondary"
							size="sm"
							href={workspacePath(slug, `/settings/teams/${listing.teamKey}`)}
						>
							Team settings
						</Button>
					</div>
				</div>
			{:else}
				<div class="flex items-baseline justify-between gap-2">
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Every cycle {listing.teamName} has run or has queued.
					</p>
					<Button
						variant="ghost"
						size="sm"
						href={workspacePath(slug, `/settings/teams/${listing.teamKey}`)}
					>
						Change cadence
					</Button>
				</div>

				{#each grouped as section (section.label)}
					<section class="flex flex-col gap-2">
						<Eyebrow class="text-ink-600">{section.label}</Eyebrow>
						<ul class="flex flex-col rounded-lg border border-line-subtle">
							{#each section.cycles as cycle (cycle.id)}
								<li
									class="cursor-row border-b border-line-subtle last:border-b-0"
									{...cursor.props(cycle)}
								>
									<a
										href={cyclePath(slug, cycle)}
										class="flex items-center justify-between gap-3 px-3 py-2.5 motion-control hover:bg-paper-2 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
									>
										<span class="flex min-w-0 flex-col gap-0.5">
											<span class="truncate text-md text-ink-900">{cycle.name}</span>
											<span class="font-mono text-xs text-muted-foreground tabular-nums">
												{cycleWindow(cycle.startsOn, cycle.endsOn)}
											</span>
										</span>
										<span class="shrink-0 text-xs text-muted-foreground">
											{phaseLabel(cycle.phase)}
										</span>
									</a>
								</li>
							{/each}
						</ul>
					</section>
				{/each}

				{#if grouped.length === 0}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						No cycles yet. The next one appears as soon as the cadence is set.
					</p>
				{/if}
			{/if}
		</div>
	</div>

	<ShortcutBar ids={["cursor-down", "cursor-open", "issue-new", "search", "help"]} />
</div>
