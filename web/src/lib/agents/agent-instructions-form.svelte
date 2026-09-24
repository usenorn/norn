<script lang="ts">
	import { get } from "svelte/store";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { agentInstructionsSchema } from "./agent-instructions-schema";

	let {
		instructions,
		locked = false,
		onsave,
	}: {
		instructions: string;
		locked?: boolean;
		onsave: (input: { instructions: string }) => Promise<boolean>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const opened = instructions;

	let held = $state(opened);
	let saved = opened;
	let arrivedLast = opened;

	function settle(value: string) {
		saved = value;
		held = value;
	}

	const form = superForm(defaults({ instructions: opened }, zod4(agentInstructionsSchema)), {
		id: "agent-instructions-form",
		SPA: true,
		validators: zod4Client(agentInstructionsSchema),
		resetForm: false,
		invalidateAll: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			if (await onsave(pending.data)) settle(pending.data.instructions);
		},
	});

	const { form: formData, enhance, submitting } = form;

	const dirty = $derived($formData.instructions !== held);

	$effect(() => {
		const arrived = instructions;

		if (arrived === arrivedLast) return;

		arrivedLast = arrived;

		if (arrived === saved) return;

		const untouched = get(formData).instructions === saved;

		settle(arrived);

		if (untouched) formData.set({ instructions: arrived });
	});
</script>

<form
	id="agent-instructions-form"
	method="POST"
	use:enhance
	class="flex flex-col gap-4 rounded-lg border border-line-strong p-4"
>
	<Form.Field {form} name="instructions">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Instructions for this agent</Form.Label>
				<Textarea
					{...props}
					rows={12}
					bind:value={$formData.instructions}
					disabled={locked || $submitting}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Saved for now and nothing else: this agent does not read them yet. When they are put to
			work, they will be added to the workspace's instructions and the project's rather than
			replace them.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<div class="flex justify-end">
		<Button type="submit" size="sm" disabled={locked || $submitting || !dirty}>
			{$submitting ? "Saving…" : "Save instructions"}
		</Button>
	</div>
</form>
