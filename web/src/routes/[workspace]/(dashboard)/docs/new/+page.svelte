<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import AudienceFields from "#lib/docs/audience-fields.svelte";
	import DocsHeader from "#lib/docs/docs-header.svelte";
	import { newDocFormId, newDocSchema } from "#lib/docs/doc-schema.js";
	import { ancestry, docFailureMessage, docPath, docsPath } from "#lib/docs/docs.js";
	import { newDocPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const known = $derived(data.tree.kind === "ready" ? data.tree.docs : []);
	const trail = $derived(data.parentDoc ? [...ancestry(known, data.parentDoc.id), data.parentDoc] : []);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: newDocFormId,
		validators: zod4Client(newDocSchema),
		resetForm: false,
	});

	const { form: formData, enhance, message, submitting } = form;

	const preview = $derived(
		import.meta.env.DEV ? newDocPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const failure = $derived(preview?.failure ?? $message);
	const busy = $derived(preview?.submitting ?? $submitting);
</script>

<svelte:head>
	<title>New doc · {data.workspace.name} · Norn</title>
</svelte:head>

<DocsHeader workspace={slug} {trail} title="New doc" />

<div class="flex-1 overflow-y-auto">
	<div class="mx-auto flex w-full max-w-120 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
		<div class="flex flex-col gap-1">
			<h2 class="text-lg font-semibold tracking-snug text-ink-900">New doc</h2>
			<p class="text-sm text-pretty text-muted-foreground">
				{#if data.parentDoc}
					It is filed under “{data.parentDoc.title}”. You write the rest once it exists.
				{:else}
					Name it and decide who reads it. You write the rest once it exists.
				{/if}
			</p>
		</div>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>The doc was not created</Alert.Title>
				<Alert.Description>{docFailureMessage(failure)}</Alert.Description>
			</Alert.Root>
		{/if}

		<form method="POST" use:enhance class="flex flex-col gap-4">
			<input type="hidden" name="workspaceId" value={data.workspace.id} />
			<input type="hidden" name="parentId" value={$formData.parentId} />

			<Form.Field {form} name="title">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Title</Form.Label>
						<Input {...props} bind:value={$formData.title} disabled={busy} autocomplete="off" />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<AudienceFields {form} projects={data.reachable} disabled={busy} />

			<div class="flex flex-wrap items-center gap-2">
				<Button type="submit" disabled={busy}>
					{busy ? "Creating" : "Create doc"}
				</Button>
				<Button
					variant="ghost"
					href={data.parentDoc ? docPath(slug, data.parentDoc.id) : docsPath(slug)}
					aria-disabled={busy}
				>
					Cancel
				</Button>
			</div>
		</form>
	</div>
</div>
