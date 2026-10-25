<script lang="ts">
	import ArrowUpRight from "@lucide/svelte/icons/arrow-up-right";
	import { evidenceLink, type MemoryEvidence } from "./memories";

	let { evidence, workspace }: { evidence: MemoryEvidence[]; workspace: string } = $props();
</script>

{#if evidence.length > 0}
	<ul class="flex flex-col gap-1.5" aria-label="Evidence">
		{#each evidence as cited, index (index)}
			{@const link = evidenceLink(workspace, cited)}
			<li class="flex min-w-0 flex-col gap-0.5 rounded-md border border-line-subtle px-2.5 py-2">
				<span class="text-2xs text-muted-foreground">{link.detail ?? link.label}</span>
				{#if link.quote !== undefined}
					<blockquote class="text-sm leading-normal text-pretty break-words text-ink-900">“{link.quote}”</blockquote>
				{:else if link.href}
					<a
						href={link.href}
						class="inline-flex min-w-0 items-center gap-1 text-sm text-link underline-offset-2 hover:text-link-hover hover:underline"
					>
						<span class="min-w-0 truncate">{link.label}</span>
						<ArrowUpRight class="size-3 shrink-0" aria-hidden="true" />
					</a>
				{:else}
					<span class="text-sm text-ink-900">{link.label}</span>
				{/if}
			</li>
		{/each}
	</ul>
{:else}
	<p class="text-xs text-muted-foreground">It cites no evidence you can read.</p>
{/if}
