<script lang="ts">
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Wrench from "@lucide/svelte/icons/wrench";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		failedPreviews,
		noPlanPreviewsLine,
		previewFixLabel,
		previewFixRequest,
		previewStateLabel,
		previewsView,
		reviewPreviewLine,
		type ReviewState,
	} from "./review";

	let {
		review,
		deciding,
		working = false,
		onfix,
	}: {
		review: ReviewState;
		deciding: boolean;
		working?: boolean;
		onfix: (request: string) => Promise<boolean>;
	} = $props();

	const view = $derived(previewsView(review));
	const failed = $derived(failedPreviews(review));
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Previews">
	<Eyebrow rule>Previews</Eyebrow>

	{#if view.kind === "none_configured"}
		<p class="text-xs text-muted-foreground text-pretty">{noPlanPreviewsLine}</p>
	{:else}
		<ul class="flex min-w-0 flex-col">
			{#each view.previews as preview (preview.name)}
				<li class="flex min-w-0 flex-col gap-1 border-b border-line-subtle py-2 last:border-b-0">
					<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
						<span class="font-mono text-xs text-ink-900">{preview.name}</span>
						<span class="text-2xs text-muted-foreground">on {preview.service}</span>
						<Eyebrow
							tone={preview.state === "ready" ? "success" : preview.state === "failed" ? "danger" : "muted"}
						>
							{previewStateLabel(preview)}
						</Eyebrow>
						<span class="flex-1"></span>
						{#if preview.url}
							<a
								href={preview.url}
								target="_blank"
								rel="noreferrer noopener"
								class="inline-flex items-center gap-1 text-xs text-ink-900 underline underline-offset-2 hover:text-foreground"
							>
								Open
								<ExternalLink aria-hidden="true" class="size-3" />
							</a>
						{/if}
					</div>
					<p class="min-w-0 text-xs break-words text-muted-foreground">{reviewPreviewLine(preview)}</p>
				</li>
			{/each}
		</ul>

		{#if deciding && failed.length > 0}
			<div>
				<Button variant="outline" size="sm" disabled={working} onclick={() => onfix(previewFixRequest(failed))}>
					<Wrench aria-hidden="true" />
					{previewFixLabel}
				</Button>
			</div>
		{/if}
	{/if}
</section>
