<script lang="ts">
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import { Button } from "#lib/components/ui/button/index.js";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import MemoryContentFields from "#lib/memories/memory-content-fields.svelte";
	import MemoryDraftStatus from "#lib/memories/memory-draft-status.svelte";
	import MemoryFormFailure from "#lib/memories/memory-form-failure.svelte";
	import { memoryEditPath, memoryPath } from "#lib/memories/memories.js";
	import { correctMemoryFormId, correctMemorySchema } from "#lib/memories/memory-schema.js";
	import { correctMemoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? correctMemoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const draft = $derived(preview?.draft ?? data.draft);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: correctMemoryFormId,
		validators: zod4Client(correctMemorySchema),
		resetForm: false,
	});

	const { form: formData, enhance, message, submitting } = form;

	const failure = $derived(preview?.failure ?? $message);
	const busy = $derived(preview?.submitting ?? $submitting);
</script>

<svelte:head>
	<title>Correct a memory · {data.workspace.name} · Norn</title>
</svelte:head>

<MemoriesHeader workspace={slug} title="Correct" />

<div class="flex-1 overflow-y-auto">
	<div class="mx-auto flex w-full max-w-150 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
		{#if draft.kind !== "ready"}
			<MemoryDraftStatus {draft} workspace={slug} workspaceName={data.workspace.name} />
		{:else}
			<div class="flex flex-col gap-1">
				<h2 class="text-lg font-semibold tracking-snug text-ink-900">Correct this memory</h2>
				<p class="text-sm text-pretty text-muted-foreground">
					A correction fixes what the memory says and keeps its history. When a new fact replaces it,
					supersede it instead.
				</p>
			</div>

			{#if failure}
				<MemoryFormFailure
					{failure}
					title="The correction was not saved"
					retry={memoryEditPath(slug, draft.memory.id)}
					workspace={slug}
					memoryId={draft.memory.id}
				/>
			{/if}

			<form method="POST" use:enhance class="flex flex-col gap-4">
				<input type="hidden" name="workspaceId" value={data.workspace.id} />
				<input type="hidden" name="expectedVersion" value={$formData.expectedVersion} />

				<MemoryContentFields {form} projects={data.projects} disabled={busy} />

				<div class="flex flex-wrap items-center gap-2">
					<Button type="submit" disabled={busy}>{busy ? "Saving" : "Save the correction"}</Button>
					<Button variant="ghost" href={memoryPath(slug, draft.memory.id)} aria-disabled={busy}>Cancel</Button>
				</div>
			</form>
		{/if}
	</div>
</div>
