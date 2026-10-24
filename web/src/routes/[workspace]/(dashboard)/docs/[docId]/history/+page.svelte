<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import RotateCcw from "@lucide/svelte/icons/rotate-ccw";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import RevisionByline from "$lib/docs/revision-byline.svelte";
	import RevisionDiff from "$lib/docs/revision-diff.svelte";
	import DocsHeader from "$lib/docs/docs-header.svelte";
	import { revisionDiff } from "$lib/docs/diff";
	import { restoreDocSchema, restoreFormId } from "$lib/docs/doc-schema";
	import {
		ancestry,
		docFailureMessage,
		docHistoryPath,
		docPath,
		docsPath,
		revisionActionLabels,
		revisionSourceLabels,
	} from "$lib/docs/docs";
	import Editor from "$lib/editor/editor.svelte";
	import { asDocument } from "$lib/editor/document";
	import { onDateAndTime } from "$lib/time";
	import { cn } from "$lib/utils.js";
	import { docHistoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const timezone = $derived(data.workspace.timezone);
	const preview = $derived(
		import.meta.env.DEV ? docHistoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const history = $derived(preview?.history ?? data.history);
	const ready = $derived(history.kind === "ready" ? history : null);
	const doc = $derived(ready?.doc ?? null);
	const known = $derived(data.tree.kind === "ready" ? data.tree.docs : []);
	const trail = $derived(doc ? [...ancestry(known, doc.id), doc] : []);
	const chosen = $derived(ready?.chosen && "revision" in ready.chosen ? ready.chosen : null);
	const snapshot = $derived(page.url.searchParams.get("view") === "snapshot");
	const diff = $derived(chosen ? revisionDiff(chosen.before?.markdown ?? "", chosen.revision.markdown) : null);
	const restorable = $derived(
		Boolean(doc?.viewer.canEdit) && doc?.status === "active" && chosen !== null && chosen.revision.version !== doc?.version
	);
	const cursor = $derived(page.url.searchParams.get("cursor"));

	// svelte-ignore state_referenced_locally
	const restore = superForm(data.restoreForm, {
		id: restoreFormId,
		validators: zod4Client(restoreDocSchema),
		resetForm: false,
	});

	const { form: restoreData, enhance, message, submitting } = restore;

	const failure = $derived(preview?.failure ?? $message);
	const restoring = $derived(preview?.restoring ?? $submitting);

	function viewPath(revisionId: string, view?: "snapshot"): string {
		const params = new URLSearchParams();

		if (cursor) params.set("cursor", cursor);
		params.set("revision", revisionId);
		if (view) params.set("view", view);

		return `${docHistoryPath(slug, doc?.id ?? "")}?${params}`;
	}

	function pagePath(next?: string): string {
		return next ? `${docHistoryPath(slug, doc?.id ?? "")}?cursor=${encodeURIComponent(next)}` : docHistoryPath(slug, doc?.id ?? "");
	}
</script>

<svelte:head>
	<title>History of {doc ? doc.title : "doc"} · {data.workspace.name} · Norn</title>
</svelte:head>

<DocsHeader workspace={slug} {trail} title="History" />

<div class="flex-1 overflow-y-auto">
	<div class="mx-auto flex w-full max-w-240 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
		{#if history.kind === "loading"}
			<div class="h-60 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
		{:else if history.kind === "not_found"}
			<div class="flex flex-col gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">No doc here</h2>
				<p class="text-sm leading-normal text-pretty text-muted-foreground">
					There is no doc at this address in {data.workspace.name}, or it is not shared with you.
				</p>
				<div>
					<Button variant="secondary" size="sm" href={docsPath(slug)}>All docs</Button>
				</div>
			</div>
		{:else if history.kind === "unavailable" || !ready || !doc}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not read the history of this doc</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else}
			<div class="flex flex-col gap-1">
				<h2 class="text-lg font-semibold tracking-snug text-ink-900">What “{doc.title}” has said</h2>
				<p class="text-sm text-pretty text-muted-foreground">
					Every version is kept. Putting one back writes it forward as a new version, so the text it
					replaces is kept too.
				</p>
			</div>

			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>It was not put back</Alert.Title>
					<Alert.Description>{docFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			<div class="grid min-w-0 gap-5 md:grid-cols-[16rem_minmax(0,1fr)] md:items-start">
				<div class="order-2 flex min-w-0 flex-col gap-2 md:order-1">
					<Eyebrow class="text-ink-600">Versions</Eyebrow>
					<ul class="flex flex-col gap-0.5" aria-label="Versions">
						{#each ready.revisions as revision (revision.id)}
							{@const current = revision.id === chosen?.revision.id}
							<li>
								<a
									href={viewPath(revision.id, snapshot ? "snapshot" : undefined)}
									aria-current={current ? "true" : undefined}
									class={cn(
										"flex min-w-0 flex-col gap-0.5 rounded-md px-2 py-1.5 motion-control hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring",
										current && "bg-accent"
									)}
								>
									<span class="flex items-center gap-1.5 text-sm text-ink-900">
										<span class="font-mono text-2xs text-muted-foreground">v{revision.version}</span>
										<span class="min-w-0 truncate">{revisionActionLabels[revision.action]}</span>
									</span>
									<span class="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
										<RevisionByline {revision} agents={ready.agents} />
									</span>
									<time datetime={revision.createdAt} class="text-2xs text-muted-foreground">
										{onDateAndTime(revision.createdAt, timezone)}
									</time>
								</a>
							</li>
						{/each}
					</ul>
					<div class="flex flex-wrap gap-2">
						{#if cursor}
							<Button variant="ghost" size="sm" href={pagePath()}>Newest versions</Button>
						{/if}
						{#if ready.nextCursor}
							<Button variant="secondary" size="sm" href={pagePath(ready.nextCursor)}>Older versions</Button>
						{/if}
					</div>
				</div>

				<section class="order-1 flex min-w-0 flex-col gap-3 md:order-2" aria-label="Chosen version">
					{#if ready.chosen && !chosen}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>We could not open this version</Alert.Title>
							<Alert.Description>Pick it again in a moment.</Alert.Description>
						</Alert.Root>
					{:else if chosen && diff}
						<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
							<Eyebrow class="text-ink-600">Version {chosen.revision.version}</Eyebrow>
							<span class="text-xs text-muted-foreground">
								{revisionSourceLabels[chosen.revision.source]}
								{#if chosen.revision.version === doc.version}· what it says now{/if}
							</span>
							<nav aria-label="How to show it" class="ml-auto flex items-center gap-1">
								<Button
									variant={snapshot ? "ghost" : "secondary"}
									size="xs"
									href={viewPath(chosen.revision.id)}
									aria-current={snapshot ? undefined : "page"}
								>
									Changes
								</Button>
								<Button
									variant={snapshot ? "secondary" : "ghost"}
									size="xs"
									href={viewPath(chosen.revision.id, "snapshot")}
									aria-current={snapshot ? "page" : undefined}
								>
									Snapshot
								</Button>
							</nav>
						</div>

						{#if chosen.before && chosen.before.title !== chosen.revision.title}
							<p class="text-sm text-ink-900">
								Retitled from “{chosen.before.title}” to “{chosen.revision.title}”.
							</p>
						{/if}

						<div class="min-w-0 overflow-hidden rounded-sm border border-line-default bg-card">
							{#if snapshot}
								<div class="p-3">
									<Editor
										document={asDocument(chosen.revision.content)}
										workspaceId={data.workspace.id}
										workspace={slug}
										disabled
										minHeight="min-h-0"
										label="What the doc said in version {chosen.revision.version}"
									/>
								</div>
							{:else}
								<RevisionDiff {diff} first={!chosen.before} />
							{/if}
						</div>

						{#if restorable}
							<form method="POST" use:enhance>
								<input type="hidden" name="workspaceId" value={data.workspace.id} />
								<input type="hidden" name="revisionId" value={$restoreData.revisionId} />
								<input type="hidden" name="expectedVersion" value={$restoreData.expectedVersion} />
								<Button type="submit" size="sm" disabled={restoring}>
									<RotateCcw aria-hidden="true" />
									{restoring ? "Putting it back" : "Put this version back"}
								</Button>
							</form>
						{:else if !doc.viewer.canEdit}
							<p class="text-xs text-muted-foreground">
								Only its author, the people who have written in it, and its maintainers can put an
								earlier version back.
							</p>
						{/if}
					{/if}
					<div>
						<Button variant="ghost" size="sm" href={docPath(slug, doc.id)}>Back to the doc</Button>
					</div>
				</section>
			</div>
		{/if}
	</div>
</div>
