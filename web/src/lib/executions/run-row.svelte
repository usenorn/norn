<script lang="ts">
	import RunState from "./run-state.svelte";
	import { onDateAndTime } from "$lib/time";
	import { cn } from "$lib/utils.js";
	import { diffStatLine, type Execution, type ExecutionChangeSummary } from "./executions";

	let {
		execution,
		href,
		timezone,
		change,
		class: className,
	}: {
		execution: Execution;
		href: string;
		timezone: string;
		change?: ExecutionChangeSummary;
		class?: string;
	} = $props();
</script>

<a
	{href}
	class={cn(
		"flex min-w-0 items-center gap-2.5 border-b border-line-subtle px-1 py-2 motion-row last:border-b-0 hover:bg-accent",
		className
	)}
>
	<span class="w-20 flex-none font-mono text-xs text-muted-foreground sm:w-24">
		{execution.reference}
	</span>
	<RunState state={execution.state} class="w-24 flex-none sm:w-32" />
	<span class="flex min-w-0 flex-1 flex-col gap-0.5">
		<span class="truncate text-xs text-ink-900">
			{execution.issueTitle || execution.agentName || "A run"}
		</span>
		<span class="truncate text-2xs text-muted-foreground">
			{execution.agentName ?? "An agent"}{execution.runnerName
				? ` on ${execution.runnerName}`
				: ""}{change && change.repositories > 0 ? ` · ${diffStatLine(change)}` : ""}
		</span>
	</span>
	<span class="flex-none font-mono text-2xs whitespace-nowrap text-muted-foreground">
		{onDateAndTime(execution.finishedAt ?? execution.queuedAt, timezone)}
	</span>
</a>
