<script lang="ts">
	import type { Instance } from "$lib/auth/instance";

	let { instance }: { instance: Instance } = $props();
</script>

{#if instance.selfHosted && instance.version}
	<p class="text-center font-mono text-xs break-all text-muted-foreground">
		{instance.host} · v{instance.version}
	</p>
{/if}
