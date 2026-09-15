<script lang="ts">
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { teamSettingsGroups } from "$lib/team/team-settings-sections";
	import { teamSettingsPath } from "$lib/team/teams";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<nav aria-label="Team settings" class="flex flex-col gap-6">
	{#each teamSettingsGroups as group (group.label)}
		<section class="flex flex-col gap-2">
			<h2 class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
				{group.label}
			</h2>
			<ul class="flex flex-col rounded-lg border border-line-default">
				{#each group.entries as entry (entry.section)}
					{@const Icon = entry.icon}
					<li class="border-b border-line-subtle last:border-b-0">
						<a
							href={teamSettingsPath(data.workspace.slug, scope.team.key, entry.section)}
							class="flex items-center gap-3 px-3 py-2.5 motion-control hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
						>
							<Icon class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
							<span class="flex min-w-0 flex-1 flex-col gap-0.5">
								<span class="text-md text-ink-900">{entry.title}</span>
								<span class="text-sm leading-normal text-muted-foreground text-pretty">
									{entry.description}
								</span>
							</span>
							<ChevronRight class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{/each}
</nav>
