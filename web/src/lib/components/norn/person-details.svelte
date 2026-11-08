<script lang="ts">
	import type { PersonDetail } from "#lib/workspace/members.js";

	let { person }: { person: PersonDetail } = $props();
</script>

<span class="flex min-w-0 flex-col gap-0.5">
	<span class="font-medium wrap-anywhere text-ink-900">{person.name}</span>
	<span class="caption text-muted-foreground">{person.standing}</span>
	{#if person.email}
		<span class="text-sm wrap-anywhere text-muted-foreground">{person.email}</span>
	{/if}
</span>
