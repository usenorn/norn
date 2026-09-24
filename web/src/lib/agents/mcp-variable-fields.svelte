<script lang="ts">
	import Plus from "@lucide/svelte/icons/plus";
	import X from "@lucide/svelte/icons/x";
	import type { SuperForm } from "sveltekit-superforms";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import type { McpServerInput } from "./agent-capability-schemas";

	type Variable = { key: string; value: string; stored: boolean };

	let {
		form,
		name,
		legend,
		description,
		keyPlaceholder,
		addLabel,
		variables = $bindable(),
		disabled,
	}: {
		form: SuperForm<McpServerInput>;
		name: "env" | "headers";
		legend: string;
		description: string;
		keyPlaceholder: string;
		addLabel: string;
		variables: Variable[];
		disabled: boolean;
	} = $props();

	function add() {
		variables = [...variables, { key: "", value: "", stored: false }];
	}

	function update(index: number, patch: Partial<Variable>) {
		variables = variables.map((variable, position) => (position === index ? { ...variable, ...patch } : variable));
	}

	function remove(index: number) {
		variables = variables.filter((_, position) => position !== index);
	}
</script>

<Form.Fieldset {form} {name} class="flex flex-col gap-2">
	<Form.Legend>{legend}</Form.Legend>
	<Form.Description>{description}</Form.Description>

	{#each variables as variable, index (index)}
		<div class="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)_auto] items-start gap-2">
			<Form.ElementField {form} name={`${name}[${index}].key`}>
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label class="sr-only">{legend} name {index + 1}</Form.Label>
						<Input
							{...props}
							value={variable.key}
							oninput={(event) => update(index, { key: event.currentTarget.value })}
							disabled={disabled || variable.stored}
							placeholder={keyPlaceholder}
							autocomplete="off"
							spellcheck="false"
							class="font-mono text-xs"
						/>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.ElementField>
			<Form.ElementField {form} name={`${name}[${index}].value`}>
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label class="sr-only">{legend} value {index + 1}</Form.Label>
						<Input
							{...props}
							type="password"
							value={variable.value}
							oninput={(event) => update(index, { value: event.currentTarget.value })}
							{disabled}
							placeholder={variable.stored ? "Stored · leave empty to keep" : "Value"}
							autocomplete="new-password"
							class="font-mono text-xs"
						/>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.ElementField>
			<Button
				type="button"
				variant="ghost"
				size="icon-sm"
				class="mt-1"
				{disabled}
				aria-label={`Remove ${variable.key || `${legend.toLowerCase()} ${index + 1}`}`}
				onclick={() => remove(index)}
			>
				<X aria-hidden="true" />
			</Button>
		</div>
	{/each}

	<Button type="button" variant="ghost" size="sm" class="w-fit" {disabled} onclick={add}>
		<Plus aria-hidden="true" />
		{addLabel}
	</Button>
	<Form.FieldErrors />
</Form.Fieldset>
