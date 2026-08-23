<script lang="ts">
	import RunState from "./run-state.svelte";
	import { onDateAndTime } from "$lib/time";
	import { cn } from "$lib/utils.js";
	import type { Execution } from "./executions";

	let {
		execution,
		href,
		timezone,
		class: className,
	}: { execution: Execution; href: string; timezone: string; class?: string } = $props();
</script>

<a
	{href}
	class={cn(
		"flex min-w-0 items-center gap-2.5 border-b border-line-subtle px-1 py-2 motion-row last:border-b-0 hover:bg-accent",
		className
	)}
>
	<span class="w-24 flex-none font-mono text-xs text-muted-foreground">
		{execution.reference}
	</span>
	<RunState state={execution.state} class="w-32 flex-none" />
	<span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">
		{execution.agentName ?? "An agent"}{execution.runnerName ? ` on ${execution.runnerName}` : ""}
	</span>
	<span class="flex-none font-mono text-2xs whitespace-nowrap text-muted-foreground">
		{onDateAndTime(execution.queuedAt, timezone)}
	</span>
</a>
