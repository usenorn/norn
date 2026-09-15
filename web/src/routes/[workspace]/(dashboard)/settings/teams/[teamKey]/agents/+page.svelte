<script lang="ts">
	import TeamAgents from "$lib/team/team-agents.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<TeamAgents
	workspace={data.workspace}
	team={scope.team}
	settings={data.agents}
	locked={scope.archived || scope.readOnly}
/>
