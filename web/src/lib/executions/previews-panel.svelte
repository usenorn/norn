<script lang="ts">
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import {
		noPreviewsLine,
		previewReach,
		previewReachLine,
		type Execution,
		type ExecutionPreview,
		type ExecutionRunner,
	} from "./executions";

	let {
		execution,
		previews,
		runner,
	}: {
		execution: Execution;
		previews: ExecutionPreview[];
		runner?: ExecutionRunner;
	} = $props();
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Previews">
	<Eyebrow rule>Previews</Eyebrow>

	{#if previews.length === 0}
		<p class="py-2 text-xs text-muted-foreground">{noPreviewsLine(execution)}</p>
	{:else}
		<ul class="flex min-w-0 flex-col">
			{#each previews as preview (preview.id)}
				{@const reach = previewReach(preview, runner)}
				<li class="flex min-w-0 flex-col gap-0.5 border-b border-line-subtle py-1.5 last:border-b-0">
					<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
						<span class="font-mono text-xs text-ink-900">{preview.name}</span>
						<span class="text-2xs text-muted-foreground">on {preview.service}</span>
						<span class="flex-1"></span>
						{#if reach.kind === "open"}
							<a
								href={reach.url}
								target="_blank"
								rel="noreferrer noopener"
								class="inline-flex items-center gap-1 text-xs text-ink-900 underline underline-offset-2 hover:text-foreground"
							>
								Open
								<ExternalLink aria-hidden="true" class="size-3" />
							</a>
						{/if}
					</div>
					<p class="min-w-0 text-xs break-all text-muted-foreground">
						{previewReachLine(reach)}
					</p>
				</li>
			{/each}
		</ul>
	{/if}
</section>
