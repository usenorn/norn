<script lang="ts">
	import Paperclip from "@lucide/svelte/icons/paperclip";
	import { Button } from "$lib/components/ui/button/index.js";

	let {
		disabled = false,
		label = "Attach",
		onfiles,
	}: { disabled?: boolean; label?: string; onfiles: (files: File[]) => void } = $props();

	let input = $state<HTMLInputElement | null>(null);

	function chosen() {
		if (!input?.files?.length) return;

		onfiles(Array.from(input.files));
		input.value = "";
	}
</script>

<Button variant="ghost" size="sm" {disabled} onclick={() => input?.click()}>
	<Paperclip aria-hidden="true" />
	{label}
</Button>

<input bind:this={input} type="file" multiple {disabled} class="sr-only" onchange={chosen} />
