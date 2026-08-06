<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { priorities } from "$lib/issues/issues";
	import { mappingSchema, type MappingInput } from "./import-schema";
	import {
		counted,
		decisionLabel,
		decisionsFor,
		grouped,
		importFailure,
		mappingKindHeading,
		mappingKindNote,
		mappingLabel,
		targetsByValue,
		targetsFor,
		type ImportDecision,
		type ImportFailure,
		type ImportMappingKind,
		type ImportMappingPlan,
		type ImportRun,
		type ImportTargets,
	} from "./imports";

	type Row = MappingInput["decisions"][number];

	let {
		workspaceId,
		run,
		plan,
		targets,
		onplan,
		onadvanced,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		plan: ImportMappingPlan;
		targets: ImportTargets;
		onplan: (plan: ImportMappingPlan) => void;
		onadvanced: () => Promise<void>;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-mapping-form";

	const form = superForm(defaults(zod4(mappingSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(mappingSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			onfailure(null);

			const decisions = entered.data.decisions.flatMap((row) =>
				row.decision === ""
					? []
					: [
							{
								kind: row.kind,
								sourceKey: row.sourceKey,
								decision: row.decision as ImportDecision,
								targetId: row.targetId || undefined,
								targetValue: row.targetValue || undefined,
							},
						]
			);

			try {
				const { data: decided, error } = await api.PUT(
					"/workspaces/{workspaceId}/imports/{importRunId}/mappings",
					{
						params: { path: { workspaceId, importRunId: run.id } },
						body: { decisions },
					}
				);

				if (error) {
					onfailure(importFailure(error));

					return;
				}

				if (!decided) {
					onfailure({ kind: "unavailable" });

					return;
				}

				if (decided.complete) {
					await onadvanced();

					return;
				}

				onplan(decided);
			} catch {
				onfailure({ kind: "unavailable" });
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const rows: Row[] = plan.mappings.map((mapping) => ({
			kind: mapping.kind,
			sourceKey: mapping.sourceKey,
			decision: mapping.decision ?? "",
			targetId: mapping.targetId ?? "",
			targetValue: mapping.targetValue ?? "",
		}));

		formData.update((entered) => ({ ...entered, decisions: rows }), { taint: false });
	});

	const groups = $derived(grouped(plan));
	const rowsByKey = $derived(
		new Map($formData.decisions.map((row) => [`${row.kind}:${row.sourceKey}`, row]))
	);
	const outstanding = $derived($formData.decisions.filter((row) => row.decision === "").length);

	function rowAt(kind: ImportMappingKind, sourceKey: string): Row | undefined {
		return rowsByKey.get(`${kind}:${sourceKey}`);
	}

	function change(kind: ImportMappingKind, sourceKey: string, patch: Partial<Row>) {
		formData.update((entered) => ({
			...entered,
			decisions: entered.decisions.map((row) =>
				row.kind === kind && row.sourceKey === sourceKey ? { ...row, ...patch } : row
			),
		}));
	}

	function fillRemaining(kind: ImportMappingKind, decision: Row["decision"]) {
		formData.update((entered) => ({
			...entered,
			decisions: entered.decisions.map((row) =>
				row.kind === kind && row.decision === ""
					? { ...row, decision, targetId: "", targetValue: "" }
					: row
			),
		}));
	}

	function acceptSuggestions(kind: ImportMappingKind) {
		const suggested = new Map(
			plan.mappings
				.filter((mapping) => mapping.kind === kind && mapping.suggestedTargetId)
				.map((mapping) => [mapping.sourceKey, mapping.suggestedTargetId as string])
		);

		formData.update((entered) => ({
			...entered,
			decisions: entered.decisions.map((row) =>
				row.kind === kind && row.decision === "" && suggested.has(row.sourceKey)
					? { ...row, decision: "map" as const, targetId: suggested.get(row.sourceKey) ?? "" }
					: row
			),
		}));
	}

	function remaining(kind: ImportMappingKind): number {
		return $formData.decisions.filter((row) => row.kind === kind && row.decision === "").length;
	}

	function suggestable(kind: ImportMappingKind): number {
		const suggested = new Set(
			plan.mappings
				.filter((mapping) => mapping.kind === kind && mapping.suggestedTargetId)
				.map((mapping) => mapping.sourceKey)
		);

		return $formData.decisions.filter(
			(row) => row.kind === kind && row.decision === "" && suggested.has(row.sourceKey)
		).length;
	}

	function targetName(kind: ImportMappingKind, row: Row): string {
		if (targetsByValue(kind)) {
			return (
				priorities.find((priority) => priority.value === row.targetValue)?.label ??
				"Choose a priority"
			);
		}

		return (
			targetsFor(kind, targets).find((option) => option.id === row.targetId)?.name ??
			"Choose what it becomes"
		);
	}
</script>

<form method="POST" id={formId} use:enhance class="flex flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">What becomes what</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Everything the source named is listed here. Nothing is imported until each one has an answer,
			and a decision applies to every issue that names it. Where Norn found something already
			carrying the same name or address it says so, but it never chooses for you.
		</p>
	</div>

	<Form.Field {form} name="decisions">
		<Form.Control>
			{#snippet children({ props })}
				<div {...props} class="flex flex-col gap-6">
					{#each groups as group (group.kind)}
						{@const left = remaining(group.kind)}
						{@const suggestions = suggestable(group.kind)}
						<section class="flex flex-col gap-3">
							<div class="flex flex-col gap-1">
								<div class="flex flex-wrap items-baseline gap-2">
									<h3 class="text-sm font-medium tracking-snug text-ink-900">
										{mappingKindHeading(group.kind)}
									</h3>
									<span class="text-xs text-muted-foreground">
										{left === 0 ? "all decided" : `${left} still to decide`}
									</span>
								</div>
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									{mappingKindNote(group.kind)}
								</p>
							</div>

							{#if left > 0}
								<div class="flex flex-wrap gap-2">
									{#if suggestions > 0}
										<Button
											type="button"
											variant="secondary"
											size="sm"
											disabled={$submitting}
											onclick={() => acceptSuggestions(group.kind)}
										>
											Use the {suggestions} suggestion{suggestions === 1 ? "" : "s"}
										</Button>
									{/if}
									{#if decisionsFor(group.kind).includes("create")}
										<Button
											type="button"
											variant="secondary"
											size="sm"
											disabled={$submitting}
											onclick={() => fillRemaining(group.kind, "create")}
										>
											Create all remaining
										</Button>
									{/if}
									{#if decisionsFor(group.kind).includes("unattributed")}
										<Button
											type="button"
											variant="secondary"
											size="sm"
											disabled={$submitting}
											onclick={() => fillRemaining(group.kind, "unattributed")}
										>
											Leave all remaining unattributed
										</Button>
									{/if}
									<Button
										type="button"
										variant="ghost"
										size="sm"
										disabled={$submitting}
										onclick={() => fillRemaining(group.kind, "skip")}
									>
										Skip all remaining
									</Button>
								</div>
							{/if}

							<ul class="flex flex-col gap-2">
								{#each group.mappings as mapping (mapping.sourceKey)}
									{@const row = rowAt(group.kind, mapping.sourceKey)}
									{#if row}
										<li
											class="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2 {row.decision ===
											''
												? 'border-warning/40'
												: 'border-line-subtle'}"
										>
											<div class="flex min-w-40 flex-1 flex-col gap-0.5">
												<span class="text-sm text-ink-900">{mappingLabel(mapping)}</span>
												{#if mapping.sourceEmail}
													<span class="font-mono text-xs text-muted-foreground">
														{mapping.sourceEmail}
													</span>
												{/if}
												{#if mapping.suggestedReason}
													<span class="text-xs text-muted-foreground">
														{mapping.suggestedReason}
													</span>
												{/if}
											</div>

											<Select.Root
												type="single"
												value={row.decision}
												disabled={$submitting}
												onValueChange={(value) =>
													change(group.kind, mapping.sourceKey, {
														decision: value as Row["decision"],
														targetId: "",
														targetValue: "",
													})}
											>
												<Select.Trigger
													class="w-48"
													aria-label="What to do with {mappingLabel(mapping)}"
												>
													{row.decision === ""
														? "Not decided"
														: decisionLabel(row.decision as ImportDecision)}
												</Select.Trigger>
												<Select.Content>
													{#each decisionsFor(group.kind) as decision (decision)}
														<Select.Item value={decision} label={decisionLabel(decision)}>
															{decisionLabel(decision)}
														</Select.Item>
													{/each}
												</Select.Content>
											</Select.Root>

											{#if row.decision === "map"}
												<Select.Root
													type="single"
													value={targetsByValue(group.kind) ? row.targetValue : row.targetId}
													disabled={$submitting}
													onValueChange={(value) =>
														change(
															group.kind,
															mapping.sourceKey,
															targetsByValue(group.kind)
																? { targetValue: value }
																: { targetId: value }
														)}
												>
													<Select.Trigger
														class="w-56"
														aria-label="What {mappingLabel(mapping)} becomes"
													>
														{targetName(group.kind, row)}
													</Select.Trigger>
													<Select.Content>
														{#if targetsByValue(group.kind)}
															{#each priorities as priority (priority.value)}
																<Select.Item value={priority.value} label={priority.label}>
																	{priority.label}
																</Select.Item>
															{/each}
														{:else}
															{#each targetsFor(group.kind, targets) as option (option.id)}
																<Select.Item value={option.id} label={option.name}>
																	{option.name}
																	{#if option.detail}
																		<span class="font-mono text-xs text-muted-foreground">
																			{option.detail}
																		</span>
																	{/if}
																</Select.Item>
															{/each}
														{/if}
													</Select.Content>
												</Select.Root>
											{/if}
										</li>
									{/if}
								{/each}
							</ul>
						</section>
					{:else}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							The source named nothing that has to be decided. Save to carry on.
						</p>
					{/each}
				</div>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<div class="flex flex-wrap items-center gap-3">
		<Form.Button disabled={$submitting}>
			{#if $submitting}
				Saving…
			{:else if outstanding === 0}
				Save and see what this would do
			{:else}
				Save what is decided
			{/if}
		</Form.Button>
		{#if outstanding > 0}
			<span class="text-sm text-muted-foreground">
				{counted(outstanding, "concept has no answer yet", "concepts have no answer yet")}
			</span>
		{/if}
	</div>
</form>
