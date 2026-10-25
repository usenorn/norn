<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { invalidate } from "$app/navigation";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "#lib/api/index.js";
	import { attempt, unknownLine } from "#lib/api/attempt.js";
	import { keys } from "#lib/api/keys.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import type { Project } from "#lib/projects/projects.js";
	import { showToast } from "#lib/toast/toasts.js";
	import AudienceFields from "./audience-fields.svelte";
	import DocReaders from "./doc-readers.svelte";
	import { shareAudienceSchema, shareFormId } from "./doc-schema";
	import {
		audienceLine,
		docFailureMessage,
		readDocFailure,
		sameAudience,
		type Doc,
		type DocAudience,
		type DocReaders as Readers,
	} from "./docs";

	let {
		open = $bindable(false),
		doc,
		readers,
		projects,
		workspaceId,
		workspaceName,
	}: {
		open?: boolean;
		doc: Doc;
		readers: Readers;
		projects: Project[];
		workspaceId: string;
		workspaceName: string;
	} = $props();

	let failure = $state("");

	const form = superForm(defaults(zod4(shareAudienceSchema)), {
		id: shareFormId,
		SPA: true,
		validators: zod4Client(shareAudienceSchema),
		resetForm: false,
		onSubmit: () => {
			failure = "";
		},
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			const audience: DocAudience =
				entered.data.visibility === "project"
					? { visibility: "project", projectId: entered.data.projectId }
					: { visibility: entered.data.visibility };

			if (sameAudience(audience, doc.audience)) {
				open = false;

				return;
			}

			const outcome = await attempt({
				run: () =>
					api.POST("/workspaces/{workspaceId}/docs/{docId}/share", {
						params: { path: { workspaceId, docId: doc.id } },
						body: { expectedVersion: doc.version, audience },
					}),
			});

			if (outcome.kind === "done") {
				open = false;
				showToast(`“${doc.title}” is shared differently now`, {
					detail: audienceLine(audience, projects, workspaceName),
				});
				await Promise.all([invalidate(keys.doc(doc.id)), invalidate(keys.docs(workspaceId))]);

				return;
			}

			if (outcome.kind === "unknown") {
				failure = unknownLine;

				return;
			}

			const read = readDocFailure(outcome.problem);

			if (read.kind === "invalid" && read.fields.includes("projectId")) {
				setError(entered, "projectId", "Choose a project you can reach.");

				return;
			}

			failure = docFailureMessage(read);
		},
	});

	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		if (!open) return;

		failure = "";
		$formData = { visibility: doc.audience.visibility, projectId: doc.audience.projectId ?? "" };
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Share “{doc.title}”</Dialog.Title>
			<Dialog.Description>
				Who reads a doc decides who finds it in search and in the docs list. Every change is kept in
				its history.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-1.5 rounded-md border border-line-subtle p-2.5">
			<Eyebrow class="text-ink-600">Who can see it now</Eyebrow>
			<DocReaders {readers} line={audienceLine(doc.audience, projects, workspaceName)} />
		</div>

		<form method="POST" use:enhance class="flex flex-col gap-4">
			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not go through</Alert.Title>
					<Alert.Description>{failure}</Alert.Description>
				</Alert.Root>
			{/if}

			<AudienceFields {form} {projects} disabled={$submitting} />

			<Dialog.Footer>
				<Button type="button" variant="ghost" disabled={$submitting} onclick={() => (open = false)}>
					Cancel
				</Button>
				<Button type="submit" disabled={$submitting}>
					{$submitting ? "Sharing" : "Share"}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
