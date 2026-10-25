<script lang="ts">
	import * as Avatar from "#lib/components/ui/avatar/index.js";

	let { class: className }: { class?: string } = $props();
</script>

<Avatar.Root size="xs" variant="ghost" role="img" aria-label="Unassigned" title="Unassigned" class={className}>
	<Avatar.Fallback aria-hidden="true">+</Avatar.Fallback>
</Avatar.Root>
