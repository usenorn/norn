<script lang="ts">
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import Link2 from "@lucide/svelte/icons/link-2";
	import X from "@lucide/svelte/icons/x";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { relationKinds, relationHeading, type IssueRelationGroup } from "$lib/issues/issues";
	import type { Issue, IssueRelationKind } from "$lib/issues/issues";

	let {
		issue,
		groups,
		candidates,
		at,
		working = false,
		onadd,
		onremove,
	}: {
		issue: Issue;
		groups: IssueRelationGroup[];
		candidates: Issue[];
		at: (path: string) => string;
		working?: boolean;
		onadd: (kind: IssueRelationKind, counterpartId: string, closeDuplicate: boolean) => void;
		onremove: (relationId: string) => void;
	} = $props();

	let chosen = $state<IssueRelationKind>("blocks");
	let closeDuplicate = $state(false);

	const related = $derived(
		new Set(groups.flatMap((group) => group.relations.map((relation) => relation.issue.id)))
	);
	const attachable = $derived(
		candidates.filter((candidate) => candidate.id !== issue.id && !related.has(candidate.id))
	);
</script>

<div class="flex flex-col gap-1.5">
	{#if groups.length === 0}
		<p class="text-md text-muted-foreground">Nothing is linked to this issue yet.</p>
	{:else}
		<ul class="flex flex-col">
			{#each groups as group (group.kind)}
					{#each group.relations as relation (relation.id)}
							<li
								class="flex items-center gap-1 border-b border-line-subtle"
							>
								<a
									href={at(`/issues/${relation.issue.reference}`)}
									class="-mx-1 flex h-row min-w-0 flex-1 items-center gap-2.25 rounded-sm px-1 transition-colors duration-70 ease-out hover:bg-accent"
								>
									<span
										class="w-19 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
									>
										{relationHeading(group.kind)}
									</span>
									<StatusIcon
										category={relation.issue.state.category}
										name={relation.issue.state.name}
									/>
									<span class="font-mono text-xs text-muted-foreground">
										{relation.issue.reference}
									</span>
									<span
										class="min-w-0 flex-1 truncate text-md {relation.issue.state.category ===
										'abandoned'
											? 'text-muted-foreground line-through decoration-line-strong'
											: 'text-ink-900'}"
									>
										{relation.issue.title}
									</span>
								</a>
								<Button
									variant="ghost"
									size="sm"
									disabled={working}
									aria-label="Unlink {relation.issue.reference}"
									onclick={() => onremove(relation.id)}
								>
									<X aria-hidden="true" />
								</Button>
							</li>
					{/each}
			{/each}
		</ul>
	{/if}

	{#if !working}
	<div class="flex flex-wrap items-center gap-1.5 pt-0.5">
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="ghost" size="sm" disabled={working}>
						<Link2 aria-hidden="true" />
						{relationKinds.find((entry) => entry.value === chosen)?.label ?? "Link"}
						<ChevronDown aria-hidden="true" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start">
				<DropdownMenu.RadioGroup
					value={chosen}
					onValueChange={(value) => (chosen = value as IssueRelationKind)}
				>
					<DropdownMenu.GroupHeading>This issue…</DropdownMenu.GroupHeading>
					{#each relationKinds as entry (entry.value)}
						<DropdownMenu.RadioItem value={entry.value}>{entry.label}</DropdownMenu.RadioItem>
					{/each}
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="ghost" size="sm" disabled={working}>
						Choose an issue
						<ChevronDown aria-hidden="true" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
				{#if attachable.length === 0}
					<DropdownMenu.Label>Nothing left to link to</DropdownMenu.Label>
				{:else}
					<DropdownMenu.Group>
						<DropdownMenu.GroupHeading>Link to</DropdownMenu.GroupHeading>
						{#each attachable as candidate (candidate.id)}
							<DropdownMenu.Item onSelect={() => onadd(chosen, candidate.id, closeDuplicate)}>
								<span class="font-mono text-xs text-muted-foreground">{candidate.reference}</span>
								<span class="truncate">{candidate.title}</span>
							</DropdownMenu.Item>
						{/each}
					</DropdownMenu.Group>
				{/if}
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		{#if chosen === "duplicates"}
			<label class="flex items-center gap-2 text-sm text-muted-foreground">
				<Checkbox bind:checked={closeDuplicate} disabled={working} />
				Also close this issue
			</label>
		{/if}
	</div>
	{/if}

</div>
