<script lang="ts">
	import Archive from "@lucide/svelte/icons/archive";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Ellipsis from "@lucide/svelte/icons/ellipsis";
	import FolderInput from "@lucide/svelte/icons/folder-input";
	import History from "@lucide/svelte/icons/history";
	import Lock from "@lucide/svelte/icons/lock";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Plus from "@lucide/svelte/icons/plus";
	import Users from "@lucide/svelte/icons/users";
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import { attempt, unknownLine } from "$lib/api/attempt";
	import { keys } from "$lib/api/keys";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Pill from "$lib/components/norn/pill.svelte";
	import DocsHeader from "$lib/docs/docs-header.svelte";
	import RevisionByline from "$lib/docs/revision-byline.svelte";
	import ShareDialog from "$lib/docs/share-dialog.svelte";
	import { archiveDocSchema, archiveFormId } from "$lib/docs/doc-schema";
	import {
		ancestry,
		descendants,
		docEditPath,
		docFailureMessage,
		docHistoryPath,
		docPath,
		docVisibilityLabels,
		newDocPath,
		projectName,
		readDocFailure,
	} from "$lib/docs/docs";
	import PropertyPicker, { type PickerOption } from "$lib/issues/property-picker.svelte";
	import Markdown from "$lib/issues/markdown.svelte";
	import { onDateAndTime } from "$lib/time";
	import { showFailure, showToast } from "$lib/toast/toasts";
	import { docPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? docPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const detail = $derived(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const doc = $derived(ready?.doc ?? null);
	const archived = $derived(doc?.status === "archived");
	const editable = $derived(Boolean(doc?.viewer.canEdit) && !archived);
	const shareable = $derived(Boolean(doc?.viewer.canShare) && !archived);
	const known = $derived(data.tree.kind === "ready" ? data.tree.docs : []);
	const trail = $derived(doc ? ancestry(known, doc.id) : []);

	let sharing = $state(false);
	let archiving = $state(false);
	let moving = $state(false);
	let moveOpen = $state(false);

	// svelte-ignore state_referenced_locally
	const archive = superForm(data.archiveForm, {
		id: archiveFormId,
		validators: zod4Client(archiveDocSchema),
		resetForm: false,
	});

	const { enhance: archiveEnhance, message: archiveMessage, submitting: archiveSubmitting } = archive;

	const homes = $derived.by((): PickerOption[] => {
		if (!doc) return [];

		const beneath = descendants(known, doc.id);

		return [
			{ value: "", label: "Top level", checked: !doc.parentId },
			...known
				.filter((candidate) => candidate.id !== doc.id && !beneath.has(candidate.id))
				.map((candidate) => ({
					value: candidate.id,
					label: candidate.title,
					checked: candidate.id === doc.parentId,
					hint: docVisibilityLabels[candidate.audience.visibility],
				})),
		];
	});

	async function moveTo(parentId: string) {
		if (!doc || moving || (parentId || undefined) === doc.parentId) return;

		moving = true;

		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/docs/{docId}/move", {
					params: { path: { workspaceId: data.workspace.id, docId: doc.id } },
					body: { expectedVersion: doc.version, ...(parentId ? { parentId } : {}) },
				}),
		});

		moving = false;

		if (outcome.kind === "done") {
			const parent = known.find((candidate) => candidate.id === parentId);

			showToast(parent ? `Moved under “${parent.title}”` : "Moved to the top level", {
				href: docPath(slug, doc.id),
			});
			await Promise.all([invalidate(keys.doc(doc.id)), invalidate(keys.docs(data.workspace.id))]);

			return;
		}

		showFailure(outcome.kind === "unknown" ? unknownLine : docFailureMessage(readDocFailure(outcome.problem)));
	}
</script>

<svelte:head>
	<title>{doc ? doc.title : "Doc"} · {data.workspace.name} · Norn</title>
</svelte:head>

<DocsHeader workspace={slug} {trail} title={doc?.title ?? "Doc"}>
	{#snippet actions()}
		{#if doc}
			{#if editable}
				<Button size="sm" href={docEditPath(slug, doc.id)}>
					<Pencil aria-hidden="true" />
					Edit
				</Button>
			{/if}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="outline" size="icon-sm" aria-label="More for this doc">
							<Ellipsis aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					{#if shareable}
						<DropdownMenu.Item onSelect={() => (sharing = true)}>
							<Users aria-hidden="true" />
							Share
						</DropdownMenu.Item>
					{/if}
					{#if !archived}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a {...props} href={newDocPath(slug, doc.id)}>
									<Plus aria-hidden="true" />
									New doc inside
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/if}
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a {...props} href={docHistoryPath(slug, doc.id)}>
								<History aria-hidden="true" />
								History
							</a>
						{/snippet}
					</DropdownMenu.Item>
					{#if editable}
						<DropdownMenu.Separator />
						<DropdownMenu.Item variant="destructive" onSelect={() => (archiving = true)}>
							<Archive aria-hidden="true" />
							Archive
						</DropdownMenu.Item>
					{/if}
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{/if}
	{/snippet}
</DocsHeader>

<div class="flex-1 overflow-y-auto">
	<article
		class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
	>
		{#if detail.kind === "loading"}
			<div class="h-8 w-2/3 animate-breathe rounded-md bg-paper-2" aria-busy="true"></div>
			<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-hidden="true"></div>
		{:else if detail.kind === "not_found"}
			<div class="flex flex-col gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">No doc here</h2>
				<p class="text-sm leading-normal text-pretty text-muted-foreground">
					There is no doc at this address in {data.workspace.name}, or it is not shared with you.
				</p>
				<div>
					<Button variant="secondary" size="sm" href="/{slug}/docs">All docs</Button>
				</div>
			</div>
		{:else if detail.kind === "unavailable" || !ready || !doc}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not load this doc</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else}
			{#if archived}
				<Alert.Root variant="warning">
					<Archive aria-hidden="true" />
					<Alert.Title>This doc is archived</Alert.Title>
					<Alert.Description>
						It stays readable, and its history is kept. Nothing about it can change any more.
					</Alert.Description>
				</Alert.Root>
			{:else if !doc.viewer.canEdit}
				<Alert.Root>
					<Lock aria-hidden="true" />
					<Alert.Title>You can read this doc</Alert.Title>
					<Alert.Description>
						Only its author, the people who have written in it, and its maintainers can change it.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<header class="flex flex-col gap-2.5">
				<h2 class="text-2xl font-semibold tracking-tight text-pretty break-words text-ink-900">{doc.title}</h2>
				<div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted-foreground">
					<Pill>
						{projectName(doc.audience, data.reachable) ?? docVisibilityLabels[doc.audience.visibility]}
					</Pill>
					{#if ready.latest}
						<span class="flex min-w-0 items-center gap-1.5">
							<RevisionByline revision={ready.latest} agents={ready.agents} />
							<span aria-hidden="true">·</span>
							<time datetime={ready.latest.createdAt}>
								{onDateAndTime(ready.latest.createdAt, data.workspace.timezone)}
							</time>
						</span>
					{/if}
					{#if editable}
						<PropertyPicker
							options={homes}
							placeholder="Move under…"
							empty="No other docs to file it under"
							bind:open={moveOpen}
							onpick={(value) => void moveTo(value)}
						>
							{#snippet trigger(props)}
								<Button {...props} variant="ghost" size="xs" disabled={moving}>
									<FolderInput aria-hidden="true" />
									{moving ? "Moving" : "Move"}
								</Button>
							{/snippet}
						</PropertyPicker>
					{/if}
				</div>
			</header>

			{#if doc.markdown.trim()}
				<Markdown source={doc.markdown} />
			{:else}
				<div class="flex flex-col items-start gap-2">
					<p class="text-md text-muted-foreground">Nothing is written here yet.</p>
					{#if editable}
						<Button variant="secondary" size="sm" href={docEditPath(slug, doc.id)}>
							<Pencil aria-hidden="true" />
							Start writing
						</Button>
					{/if}
				</div>
			{/if}
		{/if}
	</article>
</div>

{#if doc && ready}
	<ShareDialog
		bind:open={sharing}
		{doc}
		readers={ready.readers}
		projects={data.reachable}
		workspaceId={data.workspace.id}
		workspaceName={data.workspace.name}
	/>

	<AlertDialog.Root bind:open={archiving}>
		<AlertDialog.Content size="sm">
			<AlertDialog.Header>
				<AlertDialog.Title>Archive “{doc.title}”?</AlertDialog.Title>
				<AlertDialog.Description>
					It leaves the docs list and search. It stays readable from its link, and its history is
					kept.
				</AlertDialog.Description>
			</AlertDialog.Header>
			{#if $archiveMessage}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>It was not archived</Alert.Title>
					<Alert.Description>{docFailureMessage($archiveMessage)}</Alert.Description>
				</Alert.Root>
			{/if}
			<form method="POST" action="?/archive" use:archiveEnhance>
				<input type="hidden" name="workspaceId" value={data.workspace.id} />
				<input type="hidden" name="expectedVersion" value={doc.version} />
				<AlertDialog.Footer>
					<AlertDialog.Cancel type="button" disabled={$archiveSubmitting}>Cancel</AlertDialog.Cancel>
					<Button type="submit" variant="destructive" disabled={$archiveSubmitting}>
						{$archiveSubmitting ? "Archiving" : "Archive"}
					</Button>
				</AlertDialog.Footer>
			</form>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
