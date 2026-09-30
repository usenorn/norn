<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { cn } from "$lib/utils.js";
	import {
		noServicesLine,
		probeLine,
		serviceStateLabel,
		type Execution,
		type ExecutionService,
	} from "./executions";

	let {
		execution,
		services,
	}: {
		execution: Execution;
		services: ExecutionService[];
	} = $props();

	const tone = {
		starting: "text-status-not-started",
		healthy: "text-status-complete",
		unhealthy: "text-danger",
		stopped: "text-muted-foreground",
	};
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Services">
	<Eyebrow rule>Services</Eyebrow>

	{#if services.length === 0}
		<p class="py-2 text-xs text-muted-foreground">{noServicesLine(execution)}</p>
	{:else}
		<ul class="flex min-w-0 flex-col">
			{#each services as running (running.id)}
				<li class="flex min-w-0 flex-col border-b border-line-subtle py-1.5 last:border-b-0">
					<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
						<span class="font-mono text-xs text-ink-900">{running.name}</span>
						<span class={cn("text-xs", tone[running.state])}>
							{serviceStateLabel(running.state)}
						</span>
						{#if running.port > 0}
							<span class="font-mono text-2xs text-muted-foreground">port {running.port}</span>
						{/if}
						<span class="text-2xs text-muted-foreground">{probeLine(running.probe)}</span>
					</div>

					{#if running.reason}
						<p class="mt-0.5 text-xs leading-normal text-muted-foreground text-pretty">
							{running.reason}
						</p>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
