<script lang="ts">
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Hourglass from "@lucide/svelte/icons/hourglass";
	import Lock from "@lucide/svelte/icons/lock";
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
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import Pill from "#lib/components/norn/pill.svelte";
	import RowGroup from "#lib/components/norn/row-group.svelte";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import RelatedMemoryCard from "#lib/memories/related-memory.svelte";
	import {
		choicesFor,
		memoriesPath,
		memoryBasisLabels,
		memoryFailureMessage,
		memoryHold,
		memoryKindLabels,
		memoryPath,
		memoryStatusLabels,
		memoryStatusTones,
		offers,
		readMemoryFailure,
		reconciledLines,
		scopeLine,
		type Memory,
		type MemoryResolution,
	} from "#lib/memories/memories.js";
	import { onCalendarDay } from "#lib/time.js";
	import { showFailure, showToast } from "#lib/toast/toasts.js";
	import { memoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	type Confirming = { resolution: "supersede" | "no_change"; target?: Memory };

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? memoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const detail = $derived(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const memory = $derived(ready?.memory ?? null);
	const hold = $derived(ready ? memoryHold(ready.memory, ready.related) : "settled");
	const settling = $derived(Boolean(ready && memory?.viewer.canEdit && ready.choices.length > 0));

	let working = $state<MemoryResolution | null>(null);
	let confirming = $state<Confirming | null>(null);

	afterNavigate(() => {
		confirming = null;
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

	function choose(resolution: MemoryResolution, target?: Memory) {
		if (resolution === "supersede" || resolution === "no_change") {
			confirming = { resolution, target };

			return;
		}

		void reconcile(resolution, target);
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
			{#if hold === "disputed"}
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
			{/if}
			{#if !memory.viewer.canEdit}
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
					<span>{memoryBasisLabels[memory.basis]}</span>
					<span>{scopeLine(memory, data.projects)}</span>
					{#if memory.authorName}
						<span>By {memory.authorName}</span>
					{/if}
					<span>
						Holds from {onCalendarDay(memory.effectiveFrom)}{memory.effectiveUntil
							? ` to ${onCalendarDay(memory.effectiveUntil)}`
							: ""}
					</span>
				</div>
			</header>

			{#if memory.rationale}
				<section class="flex flex-col gap-1.5">
					<h3><Eyebrow rule class="text-ink-600">Why</Eyebrow></h3>
					<p class="text-sm leading-normal text-pretty break-words text-ink-900">{memory.rationale}</p>
				</section>
			{/if}

			{#if memory.conditions}
				<section class="flex flex-col gap-1.5">
					<h3><Eyebrow rule class="text-ink-600">Holds when</Eyebrow></h3>
					<p class="text-sm leading-normal text-pretty break-words text-ink-900">{memory.conditions}</p>
				</section>
			{/if}

			{#if memory.supersedesId || memory.supersededById}
				<section class="flex flex-col gap-1.5">
					<h3><Eyebrow rule class="text-ink-600">History</Eyebrow></h3>
					<div class="flex flex-wrap gap-1.5">
						{#if memory.supersedesId}
							<Button variant="secondary" size="sm" href={memoryPath(slug, memory.supersedesId)}>
								The memory it replaced
							</Button>
						{/if}
						{#if memory.supersededById}
							<Button variant="secondary" size="sm" href={memoryPath(slug, memory.supersededById)}>
								The memory that replaced it
							</Button>
						{/if}
					</div>
				</section>
			{/if}

			{#if ready.related.length > 0}
				<section class="flex flex-col gap-1.5">
					<h3><Eyebrow rule class="text-ink-600">Related memories</Eyebrow></h3>
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
				</section>
			{/if}

			{#if settling}
				<section class="flex flex-col gap-2">
					<h3><Eyebrow rule class="text-ink-600">Settle it</Eyebrow></h3>
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
				</section>
			{/if}
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
					variant={confirming?.resolution === "no_change" ? "destructive" : "default"}
					disabled={working !== null}
					onclick={() => confirming && void reconcile(confirming.resolution, confirming.target)}
				>
					{#if confirming?.resolution === "supersede"}
						{working ? "Superseding" : "Supersede"}
					{:else}
						{working ? "Dropping" : "Drop it"}
					{/if}
				</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
