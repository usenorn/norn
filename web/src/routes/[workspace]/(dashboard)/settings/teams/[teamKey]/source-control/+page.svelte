<script lang="ts">
	import TeamSourceControl from "$lib/source-control/team-source-control.svelte";
	import { statesOf } from "$lib/team/states";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<TeamSourceControl
	workspace={data.workspace}
	team={scope.team}
	rules={data.rules}
	settings={data.sourceControlSettings}
	states={statesOf(data.states)}
	locked={scope.archived || scope.readOnly}
/>
