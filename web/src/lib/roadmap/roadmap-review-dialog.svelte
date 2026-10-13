<script lang="ts">
	import { onMount } from "svelte";
	import { invalidate } from "$app/navigation";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import ReviewList from "$lib/components/norn/review-list.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import RoadmapFieldChip from "./roadmap-field-chip.svelte";
	import {
		answerOf,
		blockerMessage,
		failureMessage,
		freezeReview,
		publishFailureDialog,
		reviewFieldLabels,
		reviewSectionCopy,
		reviewSections,
		reviewToPublish,
		reviewValueLabel,
		roadmapPreviewPath,
		type ReviewDialog,
		type ReviewLine,
		type Roadmap,
		type RoadmapEntry,
		type RoadmapEntryReview,
		type RoadmapFieldChange,
	} from "./admin";

	let {
		dialog,
		entries,
		published,
		workspaceId,
		workspace,
		live,
		onchange,
		onpublished,
		oneditentry,
		oneditroadmap,
	}: {
		dialog: ReviewDialog;
		entries: RoadmapEntry[];
		published: boolean;
		workspaceId: string;
		workspace: string;
		live: boolean;
		onchange: (dialog: ReviewDialog) => void;
		onpublished: (roadmap: Roadmap) => void;
		oneditentry: (entryId: string) => void;
		oneditroadmap: () => void;
	} = $props();

	const frozen = $derived(dialog.kind === "closed" || dialog.kind === "loading" || dialog.kind === "unreadable" ? undefined : dialog.frozen);
	const sections = $derived(frozen ? reviewSections(frozen.review) : undefined);
	const blockers = $derived(frozen?.review.blockers ?? []);
	const publishing = $derived(dialog.kind === "publishing");
	const nothingToPublish = $derived(published && frozen !== undefined && !frozen.review.hasUnpublishedChanges);
	const reviewed = $derived(reviewToPublish(dialog));
	const canPublish = $derived(reviewed !== undefined && blockers.length === 0 && !nothingToPublish);

	async function read() {
		onchange({ kind: "loading" });

		const answer = await answerOf(api.GET("/workspaces/{workspaceId}/roadmap", { params: { path: { workspaceId } } }));

		onchange(answer.kind === "done" ? { kind: "ready", frozen: freezeReview(answer.roadmap) } : { kind: "unreadable", failure: answer.failure });
	}

	async function publish() {
		const frozenReview = reviewed;

		if (!canPublish || !frozenReview) return;

		onchange({ kind: "publishing", frozen: frozenReview });

		const answer = await answerOf(
			api.POST("/workspaces/{workspaceId}/roadmap/publish", {
				params: { path: { workspaceId } },
				body: { expectedVersion: frozenReview.version, expectedDigest: frozenReview.review.digest },
			})
		);

		await invalidate(keys.roadmap(workspaceId));

		if (answer.kind === "done") {
			onpublished(answer.roadmap);

			return;
		}

		onchange(publishFailureDialog(frozenReview, answer.failure));
	}

	function close() {
		if (!publishing) onchange({ kind: "closed" });
	}

	onMount(() => {
		if (live && dialog.kind === "loading") void read();
	});
</script>

{#snippet value(change: RoadmapFieldChange)}
	{#if change.before !== undefined && change.after !== undefined}
		<span class="min-w-0 break-words text-muted-foreground line-through">{reviewValueLabel(change.field, change.before)}</span>
		<span aria-hidden="true" class="text-muted-foreground">→</span>
		<span class="sr-only">becomes</span>
		<span class="min-w-0 break-words text-ink-900">{reviewValueLabel(change.field, change.after)}</span>
	{:else if change.after !== undefined}
		<span class="min-w-0 break-words text-ink-900">{reviewValueLabel(change.field, change.after)}</span>
	{:else if change.before !== undefined}
		<span class="min-w-0 break-words text-muted-foreground line-through">{reviewValueLabel(change.field, change.before)}</span>
	{/if}
{/snippet}

{#snippet entryLine(entry: RoadmapEntryReview)}
	<span class="flex w-full min-w-0 flex-col gap-1">
		<span class="text-sm break-words text-ink-900">
			{entry.title}
			<span class="font-mono text-2xs text-muted-foreground uppercase">{entry.kind}</span>
		</span>
		{#if entry.fields.length > 0}
			<span class="flex flex-col gap-1">
				{#each entry.fields as change (change.field)}
					<span class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm">
						<RoadmapFieldChip label={reviewFieldLabels[change.field]} tone={change.kind} />
						{@render value(change)}
					</span>
				{/each}
			</span>
		{/if}
	</span>
{/snippet}

{#snippet fieldLine(line: ReviewLine)}
	<span class="flex w-full min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm">
		<span class="min-w-0 break-words text-ink-900">
			{line.subject.kind === "roadmap" ? "The roadmap" : line.subject.entry.title}
		</span>
		<RoadmapFieldChip label={reviewFieldLabels[line.change.field]} tone={line.change.kind} />
		{@render value(line.change)}
	</span>
{/snippet}

<Dialog.Root
	open
	onOpenChange={(next) => {
		if (!next) close();
	}}
>
	<Dialog.Content
		variant="scrollable"
		class="sm:max-w-160"
		showCloseButton={!publishing}
		onEscapeKeydown={(event) => publishing && event.preventDefault()}
		onInteractOutside={(event) => publishing && event.preventDefault()}
	>
		<div class="flex min-h-0 flex-col gap-5 overflow-y-auto">
			<Dialog.Header>
				<Dialog.Title>Review and publish</Dialog.Title>
				<Dialog.Description>
					Exactly what publishing changes on the public roadmap, worked out from what readers see now and what they
					will see.
				</Dialog.Description>
			</Dialog.Header>

			{#if dialog.kind === "loading"}
				<div class="flex flex-col gap-3" aria-busy="true" aria-label="Working out what publishing changes">
					<Skeleton class="h-5 w-40" />
					<Skeleton class="h-24 w-full" />
					<Skeleton class="h-24 w-full" />
				</div>
			{:else if dialog.kind === "unreadable"}
				<Alert.Root variant="destructive">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>The review could not be worked out</Alert.Title>
					<Alert.Description>
						<span class="block">{failureMessage(dialog.failure)}</span>
						<span class="mt-2 block"><Button size="sm" variant="secondary" onclick={read}>Try again</Button></span>
					</Alert.Description>
				</Alert.Root>
			{:else if frozen && sections}
				<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
					{#if dialog.kind === "stale"}
						<Alert.Root variant="warning">
							<TriangleAlert aria-hidden="true" />
							<Alert.Title>This review is out of date</Alert.Title>
							<Alert.Description>
								<span class="block">{failureMessage(dialog.failure)}</span>
								<span class="mt-2 block">
									<Button size="sm" variant="secondary" onclick={read}>Review the new version</Button>
								</span>
							</Alert.Description>
						</Alert.Root>
					{:else if dialog.kind === "failed"}
						<Alert.Root variant="destructive">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>Nothing was published</Alert.Title>
							<Alert.Description>
								<span class="block">{failureMessage(dialog.failure)}</span>
								<span class="mt-2 block">
									<Button size="sm" variant="secondary" onclick={read}>Review again</Button>
								</span>
							</Alert.Description>
						</Alert.Root>
					{/if}

					{#if blockers.length > 0}
						<Alert.Root variant="destructive">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>Fix these before you publish</Alert.Title>
							<Alert.Description>
								<ul class="flex flex-col gap-2">
									{#each blockers as blocker, index (index)}
										<li class="flex flex-col items-start gap-1">
											<span>{blockerMessage(blocker, entries)}</span>
											{#if blocker.entryId && entries.some((entry) => entry.id === blocker.entryId)}
												<Button size="xs" variant="secondary" onclick={() => blocker.entryId && oneditentry(blocker.entryId)}>
													Edit the entry
												</Button>
											{:else if blocker.code === "roadmap_title_required" || blocker.code === "roadmap_title_too_long"}
												<Button size="xs" variant="secondary" onclick={oneditroadmap}>Edit the public title</Button>
											{/if}
										</li>
									{/each}
								</ul>
							</Alert.Description>
						</Alert.Root>
					{/if}
				</div>

				{#if nothingToPublish}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Nothing to publish. The public roadmap already shows exactly this draft.
					</p>
				{:else}
					<div class="flex flex-col gap-5">
						{#if sections.added.length > 0}
							<ReviewList
								heading={reviewSectionCopy.added.heading}
								note={reviewSectionCopy.added.note}
								items={sections.added}
								line={entryLine}
							/>
						{/if}
						{#if sections.changed.length > 0}
							<ReviewList
								heading={reviewSectionCopy.changed.heading}
								note={reviewSectionCopy.changed.note}
								items={sections.changed}
								line={fieldLine}
							/>
						{/if}
						{#if sections.exposed.length > 0}
							<ReviewList
								heading={reviewSectionCopy.exposed.heading}
								note={reviewSectionCopy.exposed.note}
								items={sections.exposed}
								line={fieldLine}
							/>
						{/if}
						{#if sections.concealed.length > 0}
							<ReviewList
								heading={reviewSectionCopy.concealed.heading}
								note={reviewSectionCopy.concealed.note}
								items={sections.concealed}
								line={fieldLine}
							/>
						{/if}
						{#if sections.removed.length > 0}
							<ReviewList
								heading={reviewSectionCopy.removed.heading}
								note={reviewSectionCopy.removed.note}
								items={sections.removed}
								line={entryLine}
							/>
						{/if}
					</div>
				{/if}

				<Alert.Root variant="warning">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Anyone can read and copy what you publish</Alert.Title>
					<Alert.Description>
						The public roadmap needs no sign-in. Unpublishing takes the page down, but it cannot take back copies
						people already made.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<Dialog.Footer class="items-stretch sm:items-center">
				<Button variant="ghost" href={roadmapPreviewPath(workspace)} class="sm:mr-auto" disabled={publishing}>
					Open preview
				</Button>
				<Button variant="secondary" disabled={publishing} onclick={close}>Cancel</Button>
				<Button disabled={!canPublish} onclick={publish}>
					{publishing ? "Publishing…" : published ? "Publish changes" : "Publish"}
				</Button>
			</Dialog.Footer>
		</div>
	</Dialog.Content>
</Dialog.Root>
