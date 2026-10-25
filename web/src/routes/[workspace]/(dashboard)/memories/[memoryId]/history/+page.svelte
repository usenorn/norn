<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import History from "@lucide/svelte/icons/history";
	import { page } from "$app/state";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Empty from "#lib/components/ui/empty/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import Pill from "#lib/components/norn/pill.svelte";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import MemoryEvidence from "#lib/memories/memory-evidence.svelte";
	import MemorySection from "#lib/memories/memory-section.svelte";
	import {
		memoriesPath,
		memoryBasisLabels,
		memoryHistoryPath,
		memoryKindLabels,
		memoryPath,
		memoryRevisionActionLabels,
		memoryStatusLabels,
		memoryStatusTones,
		revisionAuthorLine,
		scopeLine,
		sharedWithLine,
	} from "#lib/memories/memories.js";
	import { onCalendarDay, onDateAndTime } from "#lib/time.js";
	import { cn } from "#lib/utils.js";
	import { memoryHistoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const timezone = $derived(data.workspace.timezone);
	const preview = $derived(
		import.meta.env.DEV ? memoryHistoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const history = $derived(preview?.history ?? data.history);
	const ready = $derived(history.kind === "ready" ? history : null);
	const memory = $derived(ready?.memory ?? null);
	const chosen = $derived(ready?.chosen?.kind === "ready" ? ready.chosen.revision : null);
	const cursor = $derived(page.url.searchParams.get("cursor") ?? undefined);
</script>

<svelte:head>
	<title>History of {memory ? memory.statement : "a memory"} · {data.workspace.name} · Norn</title>
</svelte:head>

<MemoriesHeader workspace={slug} title="History" />

<div class="flex-1 overflow-y-auto">
	<div class="mx-auto flex w-full max-w-240 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
		{#if history.kind === "loading"}
			<div class="h-60 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
		{:else if history.kind === "not_found"}
			<div class="flex flex-col gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">No memory here</h2>
				<p class="text-sm leading-normal text-pretty text-muted-foreground">
					There is no memory at this address in {data.workspace.name}, or it is not shared with you.
				</p>
				<div>
					<Button variant="secondary" size="sm" href={memoriesPath(slug)}>All memories</Button>
				</div>
			</div>
		{:else if history.kind === "unavailable" || !ready || !memory}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not read the history of this memory</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else}
			<div class="flex flex-col gap-1">
				<h2 class="text-lg font-semibold tracking-snug text-pretty break-words text-ink-900">
					What “{memory.statement}” has said
				</h2>
				<p class="text-sm text-pretty text-muted-foreground">
					Every change is kept with who made it, so you can see what an agent read at any point.
				</p>
			</div>

			{#if ready.revisions.length === 0}
				<Empty.Root>
					<Empty.Media variant="icon"><History aria-hidden="true" /></Empty.Media>
					<Empty.Header>
						<Empty.Title>No changes on this page</Empty.Title>
						<Empty.Description>Go back to the newest changes to read the history from the start.</Empty.Description>
					</Empty.Header>
					<Empty.Content>
						<Button variant="secondary" size="sm" href={memoryHistoryPath(slug, memory.id)}>Newest changes</Button>
					</Empty.Content>
				</Empty.Root>
			{:else}
				<div class="grid min-w-0 gap-5 md:grid-cols-[16rem_minmax(0,1fr)] md:items-start">
					<div class="order-2 flex min-w-0 flex-col gap-2 md:order-1">
						<Eyebrow class="text-ink-600">Changes</Eyebrow>
						<ul class="flex flex-col gap-0.5" aria-label="Changes">
							{#each ready.revisions as revision (revision.id)}
								{@const current = revision.id === chosen?.id}
								<li>
									<a
										href={memoryHistoryPath(slug, memory.id, revision.id, cursor)}
										aria-current={current ? "true" : undefined}
										class={cn(
											"flex min-w-0 flex-col gap-0.5 rounded-md px-2 py-1.5 motion-control hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring",
											current && "bg-accent"
										)}
									>
										<span class="flex items-center gap-1.5 text-sm text-ink-900">
											<span class="font-mono text-2xs text-muted-foreground">v{revision.version}</span>
											<span class="min-w-0 truncate">{memoryRevisionActionLabels[revision.action]}</span>
										</span>
										<span class="min-w-0 truncate text-xs text-muted-foreground">{revisionAuthorLine(revision)}</span>
										<time datetime={revision.createdAt} class="text-2xs text-muted-foreground">
											{onDateAndTime(revision.createdAt, timezone)}
										</time>
									</a>
								</li>
							{/each}
						</ul>
						<div class="flex flex-wrap gap-2">
							{#if cursor}
								<Button variant="ghost" size="sm" href={memoryHistoryPath(slug, memory.id)}>Newest changes</Button>
							{/if}
							{#if ready.nextCursor}
								<Button
									variant="secondary"
									size="sm"
									href={memoryHistoryPath(slug, memory.id, undefined, ready.nextCursor)}
								>
									Older changes
								</Button>
							{/if}
						</div>
					</div>

					<section class="order-1 flex min-w-0 flex-col gap-4 md:order-2" aria-label="Chosen change">
						{#if ready.chosen?.kind === "unavailable"}
							<Alert.Root variant="destructive">
								<CircleX aria-hidden="true" />
								<Alert.Title>We could not open this change</Alert.Title>
								<Alert.Description>Pick it again in a moment.</Alert.Description>
							</Alert.Root>
						{:else if chosen}
							<div class="flex flex-col gap-2 rounded-md border border-line-default bg-card p-3">
								<div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted-foreground">
									<Eyebrow class="text-ink-600">Version {chosen.version}</Eyebrow>
									<Pill tone={memoryStatusTones[chosen.status]}>{memoryStatusLabels[chosen.status]}</Pill>
									<span>{memoryKindLabels[chosen.kind]}</span>
									<span>{memoryBasisLabels[chosen.basis]}</span>
									{#if chosen.version === memory.version}<span>What it says now</span>{/if}
								</div>
								<p class="text-md font-medium text-pretty break-words text-ink-900">{chosen.statement}</p>
								<p class="text-xs text-muted-foreground">
									{memoryRevisionActionLabels[chosen.action]} by {revisionAuthorLine(chosen)} ·
									<time datetime={chosen.createdAt}>{onDateAndTime(chosen.createdAt, timezone)}</time>
								</p>
							</div>

							{#if chosen.reason}
								<MemorySection title="Why it was rejected">
									<p class="text-sm leading-normal text-pretty break-words text-ink-900">{chosen.reason}</p>
								</MemorySection>
							{/if}
							{#if chosen.rationale}
								<MemorySection title="Why">
									<p class="text-sm leading-normal text-pretty break-words text-ink-900">{chosen.rationale}</p>
								</MemorySection>
							{/if}
							{#if chosen.conditions}
								<MemorySection title="Holds when">
									<p class="text-sm leading-normal text-pretty break-words text-ink-900">{chosen.conditions}</p>
								</MemorySection>
							{/if}
							<MemorySection title="Where it applied">
								<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
									<dt class="text-muted-foreground">Applies to</dt>
									<dd class="min-w-0 break-words text-ink-900">{scopeLine(chosen, data.projects)}</dd>
									<dt class="text-muted-foreground">Shared with</dt>
									<dd class="min-w-0 break-words text-ink-900">{sharedWithLine(chosen.audience, data.projects)}</dd>
									<dt class="text-muted-foreground">Holds</dt>
									<dd class="text-ink-900">
										From {onCalendarDay(chosen.effectiveFrom)}{chosen.effectiveUntil
											? ` to ${onCalendarDay(chosen.effectiveUntil)}`
											: ""}
									</dd>
								</dl>
							</MemorySection>
							<MemorySection title="Evidence">
								<MemoryEvidence evidence={chosen.evidence} workspace={slug} />
							</MemorySection>
						{/if}
						<div>
							<Button variant="ghost" size="sm" href={memoryPath(slug, memory.id)}>Back to the memory</Button>
						</div>
					</section>
				</div>
			{/if}
		{/if}
	</div>
</div>
