<script lang="ts">
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CornerLeftUp from "@lucide/svelte/icons/corner-left-up";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import type { Issue } from "$lib/issues/issues";

	let {
		issue,
		candidates,
		at,
		working = false,
		onchoose,
	}: {
		issue: Issue;
		candidates: Issue[];
		at: (path: string) => string;
		working?: boolean;
		onchoose: (parentId: string | null) => void;
	} = $props();

	const attachable = $derived(
		candidates.filter(
			(candidate) =>
				candidate.id !== issue.id &&
				candidate.status === "active" &&
				candidate.id !== issue.parentId
		)
	);
</script>

<section class="flex flex-col gap-2">
	<h2 class="text-sm font-medium text-ink-900">Parent</h2>

	<div class="flex flex-wrap items-center gap-2">
		{#if issue.parentReference}
			<a
				href={at(`/issues/${issue.parentReference}`)}
				class="inline-flex items-center gap-1.5 font-mono text-xs text-link underline-offset-2 hover:text-link-hover hover:underline"
			>
				<CornerLeftUp class="size-icon-toolbar shrink-0" aria-hidden="true" />
				{issue.parentReference}
			</a>
		{:else}
			<span class="text-sm text-muted-foreground">Not filed under anything.</span>
		{/if}

		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="outline" size="sm" disabled={working}>
						{issue.parentReference ? "Change" : "File under"}
						<ChevronDown aria-hidden="true" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
				{#if attachable.length === 0}
					<DropdownMenu.Label>No other issue can take this one</DropdownMenu.Label>
				{:else}
					<DropdownMenu.GroupHeading>File under</DropdownMenu.GroupHeading>
					{#each attachable as candidate (candidate.id)}
						<DropdownMenu.Item onSelect={() => onchoose(candidate.id)}>
							<span class="font-mono text-xs text-muted-foreground">{candidate.reference}</span>
							<span class="truncate">{candidate.title}</span>
						</DropdownMenu.Item>
					{/each}
				{/if}
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		{#if issue.parentReference}
			<Button variant="ghost" size="sm" disabled={working} onclick={() => onchoose(null)}>
				Detach
			</Button>
		{/if}
	</div>

	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		A parent and its children may sit on different teams. Moving this issue takes everything
		beneath it along, and nesting is capped at five levels.
	</p>
</section>
