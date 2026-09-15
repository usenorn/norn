<script lang="ts">
	import TeamTriage from "$lib/team/team-triage.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<TeamTriage
	workspace={data.workspace}
	team={scope.team}
	setting={data.triage}
	locked={scope.archived || scope.readOnly}
/>
