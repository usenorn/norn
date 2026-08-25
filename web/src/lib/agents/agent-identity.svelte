<script lang="ts">
	import AgentIcon from "./agent-icon.svelte";
	import type { Agent } from "./agents";

	let {
		agent,
		href,
		description,
	}: {
		agent: Agent;
		href?: string;
		description?: string;
	} = $props();
</script>

<div class="flex min-w-0 items-center gap-3">
	<span
		class="flex size-8 shrink-0 items-center justify-center rounded-md border border-line-default bg-paper-2 text-ink-600"
	>
		<AgentIcon icon={agent.icon} class="size-4" />
	</span>
	<div class="min-w-0">
		{#if href}
			<a {href} class="block truncate text-sm font-medium tracking-snug text-ink-900 hover:underline">
				{agent.name}
			</a>
		{:else}
			<p class="truncate text-sm font-medium tracking-snug text-ink-900">{agent.name}</p>
		{/if}
		{#if description}
			<p class="truncate text-xs text-muted-foreground">{description}</p>
		{/if}
	</div>
</div>
