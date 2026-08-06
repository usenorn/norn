<script lang="ts">
	import Paperclip from "@lucide/svelte/icons/paperclip";
	import { Button } from "$lib/components/ui/button/index.js";

	let {
		disabled = false,
		label = "Attach",
		iconOnly = false,
		onfiles,
	}: {
		disabled?: boolean;
		label?: string;
		iconOnly?: boolean;
		onfiles: (files: File[]) => void;
	} = $props();

	let input = $state<HTMLInputElement | null>(null);

	function chosen() {
		if (!input?.files?.length) return;

		onfiles(Array.from(input.files));
		input.value = "";
	}
</script>

<Button
	variant="ghost"
	size={iconOnly ? "icon-xs" : "sm"}
	aria-label={iconOnly ? label : undefined}
	{disabled}
	onclick={() => input?.click()}
>
	<Paperclip aria-hidden="true" />
	{#if !iconOnly}
		{label}
	{/if}
</Button>

<input bind:this={input} type="file" multiple {disabled} class="sr-only" onchange={chosen} />
