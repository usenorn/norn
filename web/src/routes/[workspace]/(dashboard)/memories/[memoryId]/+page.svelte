<script lang="ts">
	import ArrowUpRight from "@lucide/svelte/icons/arrow-up-right";
	import Ban from "@lucide/svelte/icons/ban";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import History from "@lucide/svelte/icons/history";
	import Hourglass from "@lucide/svelte/icons/hourglass";
	import Lock from "@lucide/svelte/icons/lock";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Replace from "@lucide/svelte/icons/replace";
	import Share2 from "@lucide/svelte/icons/share-2";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { afterNavigate, invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { api } from "#lib/api/index.js";
	import { attempt, unknownLine } from "#lib/api/attempt.js";
	import { keys } from "#lib/api/keys.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as AlertDialog from "#lib/components/ui/alert-dialog/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import Pill from "#lib/components/norn/pill.svelte";
	import RowGroup from "#lib/components/norn/row-group.svelte";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import MemoryReviewActions from "#lib/memories/memory-review-actions.svelte";
	import MemorySection from "#lib/memories/memory-section.svelte";
	import RejectMemoryDialog from "#lib/memories/reject-memory-dialog.svelte";
	import RelatedMemoryCard from "#lib/memories/related-memory.svelte";
	import ShareMemoryDialog from "#lib/memories/share-memory-dialog.svelte";
	import {
		choicesFor,
		evidenceLink,
		memoriesPath,
		memoryAuthorLine,
		memoryBasisLabels,
		memoryClosed,
		memoryEditPath,
		memoryFailureMessage,
		memoryHistoryPath,
		memoryHold,
		memoryKindLabels,
		memoryPath,
		memoryPending,
		memoryRevisionActionLabels,
		memoryStatusLabels,
		memoryStatusTones,
		memorySupersedePath,
		offers,
		readMemoryFailure,
		reconciledLines,
		revisionAuthorLine,
		scopeLine,
		sharedWithLine,
		type Memory,
		type MemoryFailure,
		type MemoryResolution,
	} from "#lib/memories/memories.js";
	import { refreshMemory, withdrawMemory } from "#lib/memories/memory-writes.js";
	import { onCalendarDay, onDateAndTime } from "#lib/time.js";
	import { showFailure, showToast } from "#lib/toast/toasts.js";
	import { cn } from "#lib/utils.js";
	import { memoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	type Confirming = { resolution: "supersede" | "no_change"; target?: Memory } | { resolution: "withdraw" };

	const slug = $derived(data.workspace.slug);
	const timezone = $derived(data.workspace.timezone);
	const preview = $derived(
		import.meta.env.DEV ? memoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const detail = $derived(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const memory = $derived(ready?.memory ?? null);
	const hold = $derived(ready ? memoryHold(ready.memory, ready.related) : "settled");
	const settling = $derived(Boolean(ready && memory?.viewer.canEdit && ready.choices.length > 0));
	const closed = $derived(memory ? memoryClosed(memory) : false);
	const reviewing = $derived(Boolean(memory?.viewer.canReview && memoryPending(memory)));
	const changing = $derived(Boolean(memory && !closed && (memory.viewer.canEdit || memory.viewer.canShare)));
	const supersedable = $derived(memory?.status === "accepted" || memory?.status === "disputed");
	const rejection = $derived(ready?.recent.find((revision) => revision.action === "rejected"));

	let working = $state<MemoryResolution | "withdraw" | null>(null);
	let confirming = $state<Confirming | null>(null);
	let withdrawFailure = $state<MemoryFailure | "unknown" | null>(null);
	const failure = $derived(preview?.failure ?? withdrawFailure);
	let rejectOpen = $state(false);
	let shareOpen = $state(false);

	afterNavigate(() => {
		confirming = null;
		withdrawFailure = null;
	});

	async function reconcile(resolution: MemoryResolution, target?: Memory) {
		if (!memory || working) return;

		working = resolution;

		const settled = memory;
		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/memories/{memoryId}/reconcile", {
					params: { path: { workspaceId: data.workspace.id, memoryId: settled.id } },
					body: {
						resolution,
						expectedVersion: settled.version,
						...(target ? { target: { memoryId: target.id, expectedVersion: target.version } } : {}),
					},
				}),
		});

		working = null;
		confirming = null;

		if (outcome.kind === "done") {
			showToast(reconciledLines[resolution], {
				href: memoryPath(slug, resolution === "update" && target ? target.id : settled.id),
			});
			await Promise.all([
				invalidate(keys.memory(settled.id)),
				invalidate(keys.memories(data.workspace.id)),
				...(target ? [invalidate(keys.memory(target.id))] : []),
			]);

			return;
		}

		showFailure(outcome.kind === "unknown" ? unknownLine : memoryFailureMessage(readMemoryFailure(outcome.problem)));
	}

	async function withdraw() {
		if (!memory || working) return;

		working = "withdraw";
		withdrawFailure = null;

		const withdrawn = memory;
		const outcome = await withdrawMemory(data.workspace.id, withdrawn);

		working = null;
		confirming = null;

		if (outcome.kind === "done") {
			showToast("The memory is withdrawn", { detail: "Agents no longer recall it. Its history is kept." });
			await refreshMemory(data.workspace.id, withdrawn.id);

			return;
		}

		withdrawFailure = outcome.kind === "unknown" ? "unknown" : readMemoryFailure(outcome.problem);
	}

	function choose(resolution: MemoryResolution, target?: Memory) {
		if (resolution === "supersede" || resolution === "no_change") {
			confirming = { resolution, target };

			return;
		}

		void reconcile(resolution, target);
	}

	function confirm() {
		if (!confirming) return;
		if (confirming.resolution === "withdraw") {
			void withdraw();

			return;
		}

		void reconcile(confirming.resolution, confirming.target);
	}
</script>

<svelte:head>
	<title>{memory ? memory.statement : "Memory"} · {data.workspace.name} · Norn</title>
</svelte:head>

<MemoriesHeader workspace={slug} title={memory?.statement ?? "Memory"} />

<div class="flex-1 overflow-y-auto">
	<article
		class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
	>
		{#if detail.kind === "loading"}
			<div class="h-8 w-2/3 animate-breathe rounded-md bg-paper-2" aria-busy="true"></div>
			<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-hidden="true"></div>
		{:else if detail.kind === "not_found"}
			<div class="flex flex-col gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">No memory here</h2>
				<p class="text-sm leading-normal text-pretty text-muted-foreground">
					There is no memory at this address in {data.workspace.name}, or it is not shared with you.
				</p>
				<div>
					<Button variant="secondary" size="sm" href={memoriesPath(slug)}>All memories</Button>
				</div>
			</div>
		{:else if detail.kind === "unavailable" || !ready || !memory}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not load this memory</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else}
			{#if failure}
				<Alert.Root variant={failure !== "unknown" && failure.kind === "stale" ? "warning" : "destructive"}>
					{#if failure !== "unknown" && failure.kind === "stale"}
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>This memory changed while you had it open</Alert.Title>
						<Alert.Description>
							Nothing was withdrawn. Reload to read where it stands now, then decide again.
						</Alert.Description>
						<Alert.Action>
							<Button variant="secondary" size="sm" onclick={() => refreshMemory(data.workspace.id, memory.id)}>
								Reload
							</Button>
						</Alert.Action>
					{:else}
						<CircleX aria-hidden="true" />
						<Alert.Title>It was not withdrawn</Alert.Title>
						<Alert.Description>{failure === "unknown" ? unknownLine : memoryFailureMessage(failure)}</Alert.Description>
					{/if}
				</Alert.Root>
			{/if}

			{#if memory.status === "superseded"}
				<Alert.Root variant="muted">
					<Replace aria-hidden="true" />
					<Alert.Title>Another memory replaced this one</Alert.Title>
					<Alert.Description>
						Agents recall the replacement instead. This one stays readable with its history.
					</Alert.Description>
					{#if memory.supersededById}
						<Alert.Action>
							<Button variant="secondary" size="sm" href={memoryPath(slug, memory.supersededById)}>
								Read the replacement
							</Button>
						</Alert.Action>
					{/if}
				</Alert.Root>
			{:else if memory.status === "withdrawn"}
				<Alert.Root variant="muted">
					<Trash2 aria-hidden="true" />
					<Alert.Title>This memory is withdrawn</Alert.Title>
					<Alert.Description>
						Agents no longer recall it. Anything already delivered to an agent cannot be called back.
					</Alert.Description>
				</Alert.Root>
			{:else if memory.status === "rejected"}
				<Alert.Root variant="muted">
					<Ban aria-hidden="true" />
					<Alert.Title>This memory was rejected</Alert.Title>
					<Alert.Description>
						{rejection?.reason
							? `${revisionAuthorLine(rejection)} rejected it: “${rejection.reason}”`
							: "Agents never recall it. It is kept with its history."}
					</Alert.Description>
				</Alert.Root>
			{:else if hold === "disputed"}
				<Alert.Root variant="warning">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>This contradicts an accepted decision</Alert.Title>
					<Alert.Description>
						It is held as disputed and does not guide agents until somebody settles it. The accepted
						decision keeps guiding them meanwhile.
					</Alert.Description>
				</Alert.Root>
			{:else if hold === "proposed"}
				<Alert.Root>
					<Hourglass aria-hidden="true" />
					<Alert.Title>This memory is waiting for review</Alert.Title>
					<Alert.Description>
						It repeats or qualifies a memory already recorded. Nothing changes until somebody settles
						it.
					</Alert.Description>
				</Alert.Root>
			{:else if memoryPending(memory)}
				<Alert.Root>
					<Hourglass aria-hidden="true" />
					<Alert.Title>This memory is a proposal</Alert.Title>
					<Alert.Description>
						Agents do not recall it until a workspace admin, or the lead of its project, accepts it.
					</Alert.Description>
				</Alert.Root>
			{/if}
			{#if !closed && !memory.viewer.canEdit && !memory.viewer.canReview}
				<Alert.Root>
					<Lock aria-hidden="true" />
					<Alert.Title>You can read this memory</Alert.Title>
					<Alert.Description>
						Only its author, its contributors, and its maintainers can settle or change it.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<header class="flex flex-col gap-2.5">
				<h2 class="text-2xl font-semibold tracking-tight text-pretty break-words text-ink-900">
					{memory.statement}
				</h2>
				<div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted-foreground">
					<Pill tone={memoryStatusTones[memory.status]}>{memoryStatusLabels[memory.status]}</Pill>
					<span>{memoryKindLabels[memory.kind]}</span>
					<span>{scopeLine(memory, data.projects)}</span>
					<span>Shared with {sharedWithLine(memory.audience, data.projects)}</span>
					<span>
						Holds from {onCalendarDay(memory.effectiveFrom)}{memory.effectiveUntil
							? ` to ${onCalendarDay(memory.effectiveUntil)}`
							: ""}
					</span>
				</div>
			</header>

			{#if reviewing}
				<MemorySection title="Your decision">
					<p class="text-sm leading-normal text-pretty text-muted-foreground">
						Accepted, agents recall it from now on. Rejected, it is kept with your reason and never
						recalled.
					</p>
					<MemoryReviewActions {memory} workspaceId={data.workspace.id} onreject={() => (rejectOpen = true)} />
				</MemorySection>
			{/if}

			{#if memory.rationale}
				<MemorySection title="Why">
					<p class="text-sm leading-normal text-pretty break-words text-ink-900">{memory.rationale}</p>
				</MemorySection>
			{/if}

			{#if memory.conditions}
				<MemorySection title="Holds when">
					<p class="text-sm leading-normal text-pretty break-words text-ink-900">{memory.conditions}</p>
				</MemorySection>
			{/if}

			<MemorySection title="Where it came from">
				<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
					<dt class="text-muted-foreground">Recorded by</dt>
					<dd class="min-w-0 break-words text-ink-900">{memoryAuthorLine(memory)}</dd>
					<dt class="text-muted-foreground">Basis</dt>
					<dd class="text-ink-900">{memoryBasisLabels[memory.basis]}</dd>
					<dt class="text-muted-foreground">Recorded</dt>
					<dd class="text-ink-900">
						<time datetime={memory.createdAt}>{onDateAndTime(memory.createdAt, timezone)}</time>
					</dd>
					<dt class="text-muted-foreground">Last changed</dt>
					<dd class="text-ink-900">
						<time datetime={memory.updatedAt}>{onDateAndTime(memory.updatedAt, timezone)}</time>
					</dd>
				</dl>
				{#if memory.evidence.length > 0}
					<ul class="flex flex-col gap-1.5" aria-label="Evidence">
						{#each memory.evidence as cited, index (index)}
							{@const link = evidenceLink(slug, cited)}
							<li class="flex min-w-0 flex-col gap-0.5 rounded-md border border-line-subtle px-2.5 py-2">
								<span class="text-2xs text-muted-foreground">{link.detail ?? link.label}</span>
								{#if link.quote !== undefined}
									<blockquote class="text-sm leading-normal text-pretty break-words text-ink-900">
										“{link.quote}”
									</blockquote>
								{:else if link.href}
									<a
										href={link.href}
										class="inline-flex min-w-0 items-center gap-1 text-sm text-link underline-offset-2 hover:text-link-hover hover:underline"
									>
										<span class="min-w-0 truncate">{link.label}</span>
										<ArrowUpRight class="size-3 shrink-0" aria-hidden="true" />
									</a>
								{:else}
									<span class="text-sm text-ink-900">{link.label}</span>
								{/if}
							</li>
						{/each}
					</ul>
				{:else}
					<p class="text-xs text-muted-foreground">It cites no evidence you can read.</p>
				{/if}
			</MemorySection>

			{#if ready.lineage.length > 1}
				<MemorySection title="Supersession">
					<ol class="flex flex-col gap-0.5" aria-label="Supersession chain, oldest first">
						{#each ready.lineage as link (link.id)}
							{@const current = link.id === memory.id}
							<li>
								<a
									href={memoryPath(slug, link.id)}
									aria-current={current ? "page" : undefined}
									class={cn(
										"flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 motion-control hover:bg-accent focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring",
										current && "bg-accent"
									)}
								>
									<span class="min-w-0 flex-1 truncate text-sm text-ink-900">{link.statement}</span>
									<Pill tone={memoryStatusTones[link.status]}>{memoryStatusLabels[link.status]}</Pill>
								</a>
							</li>
						{/each}
					</ol>
				</MemorySection>
			{/if}

			{#if ready.related.length > 0}
				<MemorySection title="Related memories">
					<RowGroup as="ul">
						{#each ready.related as related (related.memory.id)}
							<li>
								<RelatedMemoryCard
									{related}
									workspace={slug}
									projects={data.projects}
									offered={settling ? choicesFor(ready.choices, related.memory.id) : []}
									working={working !== null}
									onchoose={(resolution) => choose(resolution, related.memory)}
								/>
							</li>
						{/each}
					</RowGroup>
				</MemorySection>
			{/if}

			{#if settling}
				<MemorySection title="Settle it">
					<p class="text-sm leading-normal text-pretty text-muted-foreground">
						Keep it beside the related memories, fold it into one of them, let it supersede one, or
						drop it. Nothing is settled for you.
					</p>
					<div class="flex flex-wrap gap-1.5">
						{#if offers(ready.choices, "independent")}
							<Button size="sm" disabled={working !== null} onclick={() => choose("independent")}>
								<CircleCheck aria-hidden="true" />
								{working === "independent" ? "Keeping" : "Keep as its own memory"}
							</Button>
						{/if}
						{#if offers(ready.choices, "no_change")}
							<Button variant="outline" size="sm" disabled={working !== null} onclick={() => choose("no_change")}>
								<Trash2 aria-hidden="true" />
								Drop this one
							</Button>
						{/if}
					</div>
				</MemorySection>
			{/if}

			{#if changing}
				<MemorySection title="Change it">
					<div class="flex flex-wrap gap-1.5">
						{#if memory.viewer.canEdit}
							<Button variant="secondary" size="sm" href={memoryEditPath(slug, memory.id)} aria-disabled={working !== null}>
								<Pencil aria-hidden="true" />
								Correct
							</Button>
							{#if supersedable}
								<Button
									variant="secondary"
									size="sm"
									href={memorySupersedePath(slug, memory.id)}
									aria-disabled={working !== null}
								>
									<Replace aria-hidden="true" />
									Supersede
								</Button>
							{/if}
						{/if}
						{#if memory.viewer.canShare}
							<Button variant="secondary" size="sm" disabled={working !== null} onclick={() => (shareOpen = true)}>
								<Share2 aria-hidden="true" />
								Share differently
							</Button>
						{/if}
						{#if memory.viewer.canEdit}
							<Button
								variant="outline"
								size="sm"
								disabled={working !== null}
								onclick={() => (confirming = { resolution: "withdraw" })}
							>
								<Trash2 aria-hidden="true" />
								{working === "withdraw" ? "Withdrawing" : "Withdraw"}
							</Button>
						{/if}
					</div>
					<p class="text-xs leading-normal text-pretty text-muted-foreground">
						Correct it when it says the wrong thing. Supersede it when a new fact replaces it.
					</p>
				</MemorySection>
			{/if}

			<MemorySection title="History">
				{#if ready.recent.length > 0}
					<ol class="flex flex-col gap-1">
						{#each ready.recent as revision (revision.id)}
							<li class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm">
								<a
									href={memoryHistoryPath(slug, memory.id, revision.id)}
									class="text-ink-900 underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									{memoryRevisionActionLabels[revision.action]}
								</a>
								<span class="min-w-0 truncate text-xs text-muted-foreground">{revisionAuthorLine(revision)}</span>
								<time datetime={revision.createdAt} class="text-xs text-muted-foreground">
									{onDateAndTime(revision.createdAt, timezone)}
								</time>
							</li>
						{/each}
					</ol>
				{/if}
				<div>
					<Button variant="ghost" size="sm" href={memoryHistoryPath(slug, memory.id)}>
						<History aria-hidden="true" />
						Every change
					</Button>
				</div>
			</MemorySection>
		{/if}
	</article>
</div>

{#if memory}
	<AlertDialog.Root open={confirming !== null} onOpenChange={(open) => !open && (confirming = null)}>
		<AlertDialog.Content size="sm">
			{#if confirming?.resolution === "supersede" && confirming.target}
				<AlertDialog.Header>
					<AlertDialog.Title>Supersede “{confirming.target.statement}”?</AlertDialog.Title>
					<AlertDialog.Description>
						This memory is accepted in its place. The earlier one stays readable as superseded, and
						the two stay linked.
					</AlertDialog.Description>
				</AlertDialog.Header>
			{:else if confirming?.resolution === "withdraw"}
				<AlertDialog.Header>
					<AlertDialog.Title>Withdraw this memory?</AlertDialog.Title>
					<AlertDialog.Description>
						Agents stop recalling it. It is kept with its history, and what an agent already read
						cannot be called back.
					</AlertDialog.Description>
				</AlertDialog.Header>
			{:else}
				<AlertDialog.Header>
					<AlertDialog.Title>Drop this memory?</AlertDialog.Title>
					<AlertDialog.Description>
						It is withdrawn and the related memories stay as they are. Its history is kept.
					</AlertDialog.Description>
				</AlertDialog.Header>
			{/if}
			<AlertDialog.Footer>
				<AlertDialog.Cancel type="button" disabled={working !== null}>Cancel</AlertDialog.Cancel>
				<Button
					type="button"
					variant={confirming?.resolution === "supersede" ? "default" : "destructive"}
					disabled={working !== null}
					onclick={confirm}
				>
					{#if confirming?.resolution === "supersede"}
						{working ? "Superseding" : "Supersede"}
					{:else if confirming?.resolution === "withdraw"}
						{working ? "Withdrawing" : "Withdraw"}
					{:else}
						{working ? "Dropping" : "Drop it"}
					{/if}
				</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<RejectMemoryDialog bind:open={rejectOpen} {memory} workspaceId={data.workspace.id} />
	<ShareMemoryDialog
		bind:open={shareOpen}
		{memory}
		projects={data.projects}
		workspaceId={data.workspace.id}
		workspaceName={data.workspace.name}
	/>
{/if}
