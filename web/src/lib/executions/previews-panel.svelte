<script lang="ts">
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import SharePanel from "./share-panel.svelte";
	import {
		noPreviewsLine,
		previewReach,
		previewReachLine,
		type Execution,
		type ExecutionPreviewDetail,
		type ExecutionRunner,
	} from "./executions";

	let {
		execution,
		previews,
		runner,
		minted,
		working,
		now,
		timezone,
		onshare,
		onrevoke,
	}: {
		execution: Execution;
		previews: ExecutionPreviewDetail[];
		runner?: ExecutionRunner;
		minted: Record<string, string>;
		working: boolean;
		now: string;
		timezone: string;
		onshare: (name: string, lifetimeSeconds: number, passcode: string) => void;
		onrevoke: (name: string, linkId: string) => void;
	} = $props();
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Previews">
	<Eyebrow rule>Previews</Eyebrow>

	{#if previews.length === 0}
		<p class="py-2 text-xs text-muted-foreground">{noPreviewsLine(execution)}</p>
	{:else}
		<ul class="flex min-w-0 flex-col">
			{#each previews as held (held.preview.id)}
				{@const reach = previewReach(held.preview, runner)}
				<li class="flex min-w-0 flex-col gap-1 border-b border-line-subtle py-2 last:border-b-0">
					<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
						<span class="font-mono text-xs text-ink-900">{held.preview.name}</span>
						<span class="text-2xs text-muted-foreground">on {held.preview.service}</span>
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

					{#if reach.kind === "open" || reach.kind === "machine_offline"}
						<SharePanel
							preview={held.preview}
							links={held.shareLinks}
							minted={minted[held.preview.name]}
							{working}
							{now}
							{timezone}
							{onshare}
							{onrevoke}
						/>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
