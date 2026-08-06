<script lang="ts">
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { api } from "$lib/api";
	import { executeSchema } from "./import-schema";
	import PreviewLines from "./preview-lines.svelte";
	import {
		counted,
		importFailure,
		listed,
		previewLines,
		type ImportFailure,
		type ImportPreview,
		type ImportRun,
	} from "./imports";

	let {
		workspaceId,
		run,
		preview,
		teams = [],
		stale = false,
		onadvanced,
		onstale,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		preview: ImportPreview;
		teams?: string[];
		stale?: boolean;
		onadvanced: () => Promise<void>;
		onstale: () => Promise<void>;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-execute-form";

	const form = superForm(defaults(zod4(executeSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(executeSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			if (teams.length > 0 && !entered.data.acknowledgeTriage) {
				setError(entered, "acknowledgeTriage", "Agree to this before importing.");

				return;
			}

			onfailure(null);

			try {
				const { error } = await api.POST(
					"/workspaces/{workspaceId}/imports/{importRunId}/execute",
					{
						params: { path: { workspaceId, importRunId: run.id } },
						body: {
							previewDigest: entered.data.previewDigest,
							acknowledgeTriage: entered.data.acknowledgeTriage,
						},
					}
				);

				if (error) {
					const failure = importFailure(error);

					if (failure.kind === "preview_stale") {
						await onstale();

						return;
					}

					if (failure.kind === "would_triage") {
						await onadvanced();
						onfailure(failure);

						return;
					}

					onfailure(failure);

					return;
				}

				await onadvanced();
			} catch {
				onfailure({ kind: "unavailable" });
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const previewDigest = preview.digest;

		formData.update((entered) => ({ ...entered, previewDigest }), { taint: false });
	});

	const lineCount = $derived(previewLines(preview));
</script>

<form method="POST" id={formId} use:enhance class="flex flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">
			{stale ? "Read again, because this workspace changed" : "What this import would do"}
		</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			{#if stale}
				Somebody changed {run.sourceLabel || "this workspace"}'s teams, labels or states while you
				were reading, so the plan below is not the one you approved. This is the current one. Read
				it and approve it again.
			{:else}
				Nothing below has happened yet. {counted(lineCount, "line", "lines")} in all, worked out from the
				records that were read and the decisions you made. Importing applies exactly this and nothing
				else.
			{/if}
		</p>
	</div>

	<div class="flex flex-col gap-5">
		<PreviewLines
			heading="Created"
			note="Made new in this workspace, and recorded so the run can be taken back."
			lines={preview.created}
		/>
		<PreviewLines
			heading="Changed"
			note="Already here and adjusted to match the source."
			lines={preview.changed}
		/>
		<PreviewLines
			heading="Skipped"
			note="Read, but left behind — a decision said to skip it, or it names something that did not arrive."
			lines={preview.skipped}
		/>
		<PreviewLines
			heading="Unattributed"
			note="Arrives without an account behind it. The source's own name for the person stays in the text."
			lines={preview.unattributed}
		/>
		<PreviewLines
			heading="Worth knowing"
			note="Things that do not stop the import but do not survive it whole."
			lines={preview.warnings}
		/>
	</div>

	{#if teams.length > 0}
		<div class="flex flex-col gap-3 rounded-lg border border-warning/40 p-4">
			<div class="flex flex-col gap-1">
				<h3 class="text-sm font-medium tracking-snug text-ink-900">
					{listed(teams)}
					{teams.length === 1 ? "puts" : "put"} new issues in triage
				</h3>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Everything this import creates on {teams.length === 1 ? "that team" : "those teams"} lands
					in triage rather than the backlog, and somebody has to accept each one before it appears
					on the board. On a backlog this size that is a lot of accepting. Turn triage off on
					{teams.length === 1 ? "that team" : "those teams"} first if that is not what you want.
				</p>
			</div>

			<Form.Field {form} name="acknowledgeTriage">
				<Form.Control>
					{#snippet children({ props })}
						<div class="flex items-start gap-2">
							<Checkbox
								{...props}
								disabled={$submitting}
								checked={$formData.acknowledgeTriage}
								onCheckedChange={(checked) => ($formData.acknowledgeTriage = checked === true)}
							/>
							<Form.Label>
								I understand imported issues land in triage on {listed(teams)}
							</Form.Label>
						</div>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>
		</div>
	{/if}

	<div>
		<Form.Button disabled={$submitting}>
			{#if $submitting}
				Starting the import…
			{:else if stale}
				Import what is above now
			{:else}
				Import this
			{/if}
		</Form.Button>
	</div>
</form>
