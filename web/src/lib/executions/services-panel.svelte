<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { atClock } from "$lib/time";
	import { cn } from "$lib/utils.js";
	import {
		logsFor,
		noServicesLine,
		probeLine,
		serviceStateLabel,
		type Execution,
		type ExecutionLogEntry,
		type ExecutionService,
	} from "./executions";

	let {
		execution,
		services,
		logs,
		timezone,
	}: {
		execution: Execution;
		services: ExecutionService[];
		logs: ExecutionLogEntry[];
		timezone: string;
	} = $props();

	let opened = $state("");

	const tone = {
		starting: "text-status-not-started",
		healthy: "text-status-complete",
		unhealthy: "text-danger",
		stopped: "text-muted-foreground",
	};

	function toggle(name: string) {
		opened = opened === name ? "" : name;
	}
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Services">
	<Eyebrow rule>Services</Eyebrow>

	{#if services.length === 0}
		<p class="py-2 text-xs text-muted-foreground">{noServicesLine(execution)}</p>
	{:else}
		<ul class="flex min-w-0 flex-col">
			{#each services as running (running.id)}
				{@const lines = logsFor(logs, running.name)}
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
						<span class="flex-1"></span>
						<Button
							variant="ghost"
							size="sm"
							aria-expanded={opened === running.name}
							onclick={() => toggle(running.name)}
						>
							{opened === running.name ? "Hide output" : "Output"}
						</Button>
					</div>

					{#if running.reason}
						<p class="mt-0.5 text-xs leading-normal text-muted-foreground text-pretty">
							{running.reason}
						</p>
					{/if}

					{#if opened === running.name}
						{#if lines.length === 0}
							<p class="mt-1.5 text-xs text-muted-foreground">
								This service has printed nothing norn is holding.
							</p>
						{:else}
							<div class="mt-1.5 max-h-64 overflow-auto rounded-sm bg-paper-1 p-2">
								<ol class="flex min-w-0 flex-col gap-0.5">
									{#each lines as line, index (index)}
										<li class="flex min-w-0 gap-2 font-mono text-2xs">
											{#if line.at}
												<time class="flex-none text-muted-foreground" datetime={line.at}>
													{atClock(line.at, timezone)}
												</time>
											{/if}
											<span class="min-w-0 flex-1 break-all whitespace-pre-wrap text-ink-900">
												{line.text}
											</span>
										</li>
									{/each}
								</ol>
							</div>
						{/if}
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
