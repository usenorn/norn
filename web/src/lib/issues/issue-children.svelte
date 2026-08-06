<script lang="ts">
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import type { Issue } from "$lib/issues/issues";

	let {
		children,
		at,
	}: {
		children: Issue[];
		at: (path: string) => string;
	} = $props();
</script>

{#if children.length === 0}
	<p class="text-md text-muted-foreground">Nothing is filed under this issue yet.</p>
{:else}
	<ul class="flex flex-col">
		{#each children as child (child.id)}
			<li class="border-b border-line-subtle">
				<a
					href={at(`/issues/${child.reference}`)}
					class="-mx-1 flex h-row items-center gap-2.25 rounded-sm px-1 transition-colors duration-70 ease-out hover:bg-accent"
				>
					<StatusIcon category={child.state.category} name={child.state.name} />
					<span class="font-mono text-xs text-muted-foreground">{child.reference}</span>
					<span
						class="min-w-0 flex-1 truncate text-md {child.state.category === 'complete'
							? 'text-muted-foreground line-through decoration-line-strong'
							: 'text-ink-900'}"
					>
						{child.title}
					</span>
					<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
						{child.state.name}
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
