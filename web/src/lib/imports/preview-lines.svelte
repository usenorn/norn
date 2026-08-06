<script lang="ts">
	import { outcomeLabel, resourceLabel, type ImportPreviewLine } from "./imports";

	let {
		heading,
		note,
		lines,
		shown = 25,
	}: {
		heading: string;
		note: string;
		lines: ImportPreviewLine[];
		shown?: number;
	} = $props();

	let expanded = $state(false);

	const visible = $derived(expanded ? lines : lines.slice(0, shown));
	const hidden = $derived(lines.length - visible.length);
</script>

<section class="flex flex-col gap-2">
	<div class="flex flex-col gap-0.5">
		<h3 class="text-sm font-medium tracking-snug text-ink-900">
			{heading}
			<span class="font-mono text-xs text-muted-foreground">{lines.length}</span>
		</h3>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">{note}</p>
	</div>

	{#if lines.length === 0}
		<p class="text-sm text-muted-foreground">Nothing.</p>
	{:else}
		<ul class="flex flex-col gap-1 rounded-md border border-line-subtle bg-paper-0 p-3">
			{#each visible as line, index (index)}
				<li class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="font-mono text-xs text-muted-foreground">{resourceLabel(line.resource)}</span>
					<span class="text-sm text-ink-900">{line.subject ?? line.externalId ?? "—"}</span>
					<span class="text-xs text-muted-foreground">{outcomeLabel(line.outcome)}</span>
					{#if line.detail}
						<span class="text-xs text-muted-foreground text-pretty">{line.detail}</span>
					{/if}
				</li>
			{/each}
		</ul>

		{#if hidden > 0}
			<div>
				<button
					type="button"
					class="text-sm text-link underline-offset-2 hover:underline"
					onclick={() => (expanded = true)}
				>
					Show the other {hidden}
				</button>
			</div>
		{/if}
	{/if}
</section>
