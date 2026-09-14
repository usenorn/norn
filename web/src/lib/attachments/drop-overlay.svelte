<script lang="ts">
	let { inset = "inset-2" }: { inset?: string } = $props();
</script>

<div
	class="pointer-events-none absolute {inset} z-10 flex items-center justify-center rounded-lg border-2 border-dashed border-primary bg-paper-0/85 text-md text-ink-900"
>
	Drop files to attach them
</div>
