<script lang="ts">
	import MoreHorizontal from "@lucide/svelte/icons/more-horizontal";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Puzzle from "@lucide/svelte/icons/puzzle";
	import RefreshCw from "@lucide/svelte/icons/refresh-cw";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import Unplug from "@lucide/svelte/icons/unplug";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { skillSummary, type AgentSkill, type SkillAction } from "./agent-capabilities";

	let {
		skill,
		shared,
		usedBy,
		busy,
		canManage,
		canDetach,
		onaction,
	}: {
		skill: AgentSkill;
		shared: boolean;
		usedBy?: string[];
		busy: boolean;
		canManage: boolean;
		canDetach: boolean;
		onaction: (action: SkillAction) => void;
	} = $props();

	const hasActions = $derived(shared ? canDetach : canManage);
</script>

<li class="flex min-w-0 items-start gap-3 p-3" aria-busy={busy}>
	<Puzzle class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
	<div class="min-w-0 flex-1">
		<div class="flex flex-wrap items-center gap-2">
			<p class="text-sm text-ink-900">{skill.name}</p>
			{#if shared}
				<Tag name="Library" color="violet" />
			{/if}
			<Tag name={skill.source === "github" ? "Imported" : "Written"} />
		</div>
		<p class="mt-0.5 line-clamp-2 text-xs leading-normal text-muted-foreground text-pretty">{skill.description}</p>
		<p class="mt-1 font-mono text-2xs break-all text-muted-foreground">{skillSummary(skill)}</p>
		{#if usedBy}
			<p class="mt-1 text-xs text-muted-foreground">
				{usedBy.length === 0 ? "No agent uses it yet" : `Used by ${usedBy.join(", ")}`}
			</p>
		{/if}
	</div>
	{#if hasActions}
		<DropdownMenu.Root>
			<DropdownMenu.Trigger disabled={busy}>
				{#snippet child({ props })}
					<Button {...props} variant="ghost" size="icon-sm" aria-label={`Actions for ${skill.name}`}>
						<MoreHorizontal aria-hidden="true" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end">
				{#if shared}
					<DropdownMenu.Item onSelect={() => onaction("detach")}>
						<Unplug aria-hidden="true" />
						Stop using
					</DropdownMenu.Item>
				{:else}
					{#if skill.source === "github"}
						<DropdownMenu.Item onSelect={() => onaction("pull")}>
							<RefreshCw aria-hidden="true" />
							Update from source
						</DropdownMenu.Item>
					{:else}
						<DropdownMenu.Item onSelect={() => onaction("edit")}>
							<Pencil aria-hidden="true" />
							Edit
						</DropdownMenu.Item>
					{/if}
					<DropdownMenu.Item variant="destructive" onSelect={() => onaction("delete")}>
						<Trash2 aria-hidden="true" />
						Delete
					</DropdownMenu.Item>
				{/if}
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	{/if}
</li>
