<script lang="ts">
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import { Button } from "#lib/components/ui/button/index.js";
	import AudienceFields from "#lib/docs/audience-fields.svelte";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import MemoryContentFields from "#lib/memories/memory-content-fields.svelte";
	import MemoryDraftStatus from "#lib/memories/memory-draft-status.svelte";
	import MemoryFormFailure from "#lib/memories/memory-form-failure.svelte";
	import MemorySection from "#lib/memories/memory-section.svelte";
	import { memoryPath, memorySupersedePath } from "#lib/memories/memories.js";
	import { supersedeMemoryFormId, supersedeMemorySchema } from "#lib/memories/memory-schema.js";
	import { supersedeMemoryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? supersedeMemoryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const draft = $derived(preview?.draft ?? data.draft);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: supersedeMemoryFormId,
		validators: zod4Client(supersedeMemorySchema),
		resetForm: false,
	});

	const { form: formData, enhance, message, submitting } = form;

	const failure = $derived(preview?.failure ?? $message);
	const busy = $derived(preview?.submitting ?? $submitting);
</script>

<svelte:head>
	<title>Supersede a memory · {data.workspace.name} · Norn</title>
</svelte:head>

<MemoriesHeader workspace={slug} title="Supersede" />

<div class="flex-1 overflow-y-auto">
	<div class="mx-auto flex w-full max-w-150 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
		{#if draft.kind !== "ready"}
			<MemoryDraftStatus {draft} workspace={slug} workspaceName={data.workspace.name} />
		{:else}
			<div class="flex flex-col gap-1">
				<h2 class="text-lg font-semibold tracking-snug text-ink-900">Record what replaces it</h2>
				<p class="text-sm text-pretty text-muted-foreground">
					The replacement is a new memory linked to this one. Once it is accepted, this one is kept as
					superseded and agents recall the replacement instead.
				</p>
			</div>

			<MemorySection title="It replaces">
				<a
					href={memoryPath(slug, draft.memory.id)}
					class="text-sm text-pretty break-words text-ink-900 underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				>
					{draft.memory.statement}
				</a>
			</MemorySection>

			{#if failure}
				<MemoryFormFailure
					{failure}
					title="The replacement was not recorded"
					retry={memorySupersedePath(slug, draft.memory.id)}
					workspace={slug}
					memoryId={draft.memory.id}
				/>
			{/if}

			<form method="POST" use:enhance class="flex flex-col gap-4">
				<input type="hidden" name="workspaceId" value={data.workspace.id} />
				<input type="hidden" name="expectedVersion" value={$formData.expectedVersion} />

				<MemoryContentFields {form} projects={data.projects} disabled={busy} />
				<AudienceFields {form} projects={data.projects} disabled={busy} />

				<div class="flex flex-wrap items-center gap-2">
					<Button type="submit" disabled={busy}>{busy ? "Recording" : "Record the replacement"}</Button>
					<Button variant="ghost" href={memoryPath(slug, draft.memory.id)} aria-disabled={busy}>Cancel</Button>
				</div>
			</form>
		{/if}
	</div>
</div>
