<script lang="ts" generics="T extends Record<string, unknown>, U extends FormPath<T>">
	import type { Snippet } from "svelte";
	import type { FormPath, SuperForm } from "sveltekit-superforms";
	import * as Form from "$lib/components/ui/form/index.js";

	let {
		form,
		name,
		label,
		hint,
		control,
	}: {
		form: SuperForm<T>;
		name: U;
		label: string;
		hint?: string;
		control: Snippet<[Record<string, unknown>]>;
	} = $props();
</script>

<Form.Field
	{form}
	{name}
	class="grid gap-x-4 gap-y-2 px-3 py-2.5 sm:min-h-11 sm:grid-cols-[minmax(0,1fr)_12rem] sm:items-center"
>
	<Form.Control>
		{#snippet children({ props })}
			<div class="flex min-w-0 flex-col gap-0.5">
				<Form.Label>{label}</Form.Label>
				{#if hint}
					<Form.Description>{hint}</Form.Description>
				{/if}
			</div>
			{@render control(props)}
		{/snippet}
	</Form.Control>
	<Form.FieldErrors class="sm:col-span-2" />
</Form.Field>
