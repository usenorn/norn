<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		agentExecutionHints,
		agentExecutionLabels,
		agentExecutions,
		type AgentExecution,
	} from "./agents";
	import { agentExecutionSchema } from "./agent-execution-schema";

	let {
		execution,
		locked = false,
		onsave,
	}: {
		execution: AgentExecution;
		locked?: boolean;
		onsave: (input: { execution: AgentExecution }) => Promise<boolean>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const opened = { execution };

	let held = $state(opened);

	const form = superForm(defaults(opened, zod4(agentExecutionSchema)), {
		id: "agent-execution-form",
		SPA: true,
		validators: zod4Client(agentExecutionSchema),
		resetForm: false,
		invalidateAll: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			if (await onsave(pending.data)) held = { ...pending.data };
		},
	});

	const { form: formData, enhance, submitting } = form;

	const dirty = $derived($formData.execution !== held.execution);
</script>

<form
	id="agent-execution-form"
	method="POST"
	use:enhance
	class="flex flex-col gap-4 rounded-lg border border-line-strong p-4"
>
	<Form.Fieldset {form} name="execution">
		<Form.Legend>Where it works</Form.Legend>
		<RadioGroup.Root name="execution" bind:value={$formData.execution} disabled={locked || $submitting}>
			{#each agentExecutions as offer (offer)}
				<div class="flex items-start gap-2">
					<RadioGroup.Item id={`agent-execution-${offer}`} value={offer} class="mt-0.5" />
					<label for={`agent-execution-${offer}`} class="flex flex-col gap-0.5">
						<span class="text-sm leading-normal text-ink-900">{agentExecutionLabels[offer]}</span>
						<span class="text-xs leading-normal text-ink-400">{agentExecutionHints[offer]}</span>
					</label>
				</div>
			{/each}
		</RadioGroup.Root>
		<Form.FieldErrors />
	</Form.Fieldset>

	<div class="flex justify-end">
		<Button type="submit" size="sm" disabled={locked || $submitting || !dirty}>
			{$submitting ? "Saving…" : "Save execution"}
		</Button>
	</div>
</form>
