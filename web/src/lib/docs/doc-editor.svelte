<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { beforeNavigate, invalidate } from "$app/navigation";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import { attempt } from "$lib/api/attempt";
	import { keys } from "$lib/api/keys";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Autosave, type SaveOutcome, type SaveState } from "$lib/editor/autosave.svelte";
	import { asDocument, sameDocument, withParagraph, type Document } from "$lib/editor/document";
	import EditConflict from "$lib/editor/edit-conflict.svelte";
	import SaveStatus from "$lib/editor/save-status.svelte";
	import Editor from "$lib/editor/editor.svelte";
	import { docTitleFormId, docTitleFormSchema, docTitleSchema } from "./doc-schema";
	import { docFailureMessage, readDocFailure, type Doc, type DocFailure } from "./docs";

	let {
		doc,
		workspaceId,
		workspace,
		shownSave,
		shownConflict,
	}: {
		doc: Doc;
		workspaceId: string;
		workspace: string;
		shownSave?: SaveState;
		shownConflict?: { mine: Document; theirs: Document };
	} = $props();

	type Written = { title: string; content: Document };

	// svelte-ignore state_referenced_locally
	let content = $state.raw<Document>(withParagraph(asDocument(doc.content)));
	// svelte-ignore state_referenced_locally
	let base = $state.raw<Written>({ title: doc.title, content: asDocument(doc.content) });
	// svelte-ignore state_referenced_locally
	let version = $state(doc.version);
	let conflicted = $state.raw<Document | null>(null);
	let refusal = $state.raw<DocFailure | null>(null);

	// svelte-ignore state_referenced_locally
	const titleForm = superForm(defaults({ title: doc.title }, zod4(docTitleFormSchema)), {
		id: docTitleFormId,
		SPA: true,
		validators: zod4Client(docTitleFormSchema),
		validationMethod: "oninput",
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (entered.valid) await autosave.flush();
		},
	});

	const { form: titleData, enhance } = titleForm;

	const titled = $derived(docTitleSchema.safeParse($titleData.title));
	const changed = $derived(
		(titled.success && titled.data !== base.title) || !sameDocument(content, base.content)
	);

	const autosave = new Autosave(save);

	const conflict = $derived(
		shownConflict ?? (conflicted ? { mine: conflicted, theirs: asDocument(doc.content) } : null)
	);

	async function save(): Promise<SaveOutcome> {
		if (conflicted) return "conflict";
		if (!titled.success) return "failed";

		const sent: Written = { title: titled.data, content };
		const retitled = sent.title !== base.title;
		const rewritten = !sameDocument(sent.content, base.content);

		if (!retitled && !rewritten) return "saved";

		refusal = null;

		const outcome = await attempt({
			run: () =>
				api.PATCH("/workspaces/{workspaceId}/docs/{docId}", {
					params: { path: { workspaceId, docId: doc.id } },
					body: {
						expectedVersion: version,
						...(retitled ? { title: sent.title } : {}),
						...(rewritten ? { content: sent.content } : {}),
					},
				}),
		});

		if (outcome.kind === "done") {
			base = sent;
			version = outcome.value.version;
			void invalidate(keys.doc(doc.id));
			if (retitled) void invalidate(keys.docs(workspaceId));

			return "saved";
		}

		if (outcome.kind === "unknown") return "unknown";

		const failure = readDocFailure(outcome.problem);

		if (failure.kind === "stale") {
			conflicted = sent.content;
			await invalidate(keys.doc(doc.id));

			return "conflict";
		}

		refusal = failure;

		return "failed";
	}

	function keepMine() {
		base = { title: doc.title, content: asDocument(doc.content) };
		version = doc.version;
		conflicted = null;
		autosave.rest();
		void autosave.flush();
	}

	function takeTheirs() {
		const theirs = asDocument(doc.content);

		content = withParagraph(theirs);
		$titleData.title = doc.title;
		base = { title: doc.title, content: theirs };
		version = doc.version;
		conflicted = null;
		autosave.rest();
	}

	$effect(() => {
		if (!changed || conflicted || !titled.success) return;

		autosave.schedule();
	});

	beforeNavigate(() => {
		if (changed && !conflicted && titled.success) void autosave.flush();
	});

	const standing = $derived(shownSave ?? autosave.state);
</script>

<div class="flex flex-col gap-4">
	<div class="flex min-h-control-sm flex-wrap items-center gap-2">
		<SaveStatus {standing} onretry={() => void autosave.flush()} />
	</div>

	{#if refusal}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Your last change was not saved</Alert.Title>
			<Alert.Description>{docFailureMessage(refusal)}</Alert.Description>
		</Alert.Root>
	{/if}

	<form method="POST" use:enhance>
		<Form.Field form={titleForm} name="title">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label class="sr-only">Title</Form.Label>
					<Input
						{...props}
						bind:value={$titleData.title}
						variant="title"
						placeholder="Untitled"
						autocomplete="off"
						disabled={standing.kind === "conflict"}
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
	</form>

	{#if conflict}
		<EditConflict
			mine={conflict.mine}
			theirs={conflict.theirs}
			{workspaceId}
			{workspace}
			summary="Somebody saved a different version of this doc while you were writing. Neither has been thrown away."
			mineLabel="The doc as you wrote it"
			theirsLabel="The doc as it was saved"
			busy={standing.kind === "saving"}
			onkeep={keepMine}
			ontake={takeTheirs}
		/>
	{/if}

	<Editor
		bind:document={content}
		{workspaceId}
		{workspace}
		label="Doc"
		placeholder="Write, or type / for blocks…"
		minHeight="min-h-80"
		disabled={Boolean(conflict)}
		autofocus
	/>
</div>
