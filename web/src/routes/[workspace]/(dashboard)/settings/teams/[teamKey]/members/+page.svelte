<script lang="ts">
	import { page } from "$app/state";
	import TeamMembers from "$lib/team/team-members.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { teamMembersPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();

	const preview = $derived(
		import.meta.env.DEV
			? teamMembersPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
</script>

<section class="flex flex-col gap-4">
	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		Belonging to {data.workspace.name} does not put someone on this team.
	</p>

	<TeamMembers
		workspace={{ id: data.workspace.id, name: data.workspace.name, slug: data.workspace.slug }}
		team={{ id: scope.team.id, name: scope.team.name }}
		roster={preview?.roster ?? data.roster}
		failure={preview?.failure ?? null}
		readOnly={scope.readOnly}
		archived={scope.archived}
	/>
</section>
