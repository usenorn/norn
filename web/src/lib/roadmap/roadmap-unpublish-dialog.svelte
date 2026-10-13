<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";

	let {
		open,
		unpublishing,
		onconfirm,
		oncancel,
	}: { open: boolean; unpublishing: boolean; onconfirm: () => void; oncancel: () => void } = $props();
</script>

<AlertDialog.Root
	{open}
	onOpenChange={(next) => {
		if (!next && !unpublishing) oncancel();
	}}
>
	<AlertDialog.Content size="sm">
		<AlertDialog.Header>
			<Eyebrow tone="danger">Takes effect immediately</Eyebrow>
			<AlertDialog.Title>Take the public roadmap down</AlertDialog.Title>
			<AlertDialog.Description>
				The public page stops showing anything at once. Your draft stays as it is, so you can publish it again
				later.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Copies people already made of the public page, such as screenshots or saved pages, cannot be taken back.
		</p>

		<AlertDialog.Footer>
			{#if unpublishing}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action variant="destructive" disabled={unpublishing} onclick={onconfirm}>
				{unpublishing ? "Unpublishing" : "Unpublish"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
