<script lang="ts">
	import TeamNotifications from "$lib/notifications/team-notifications.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<TeamNotifications
	workspace={data.workspace}
	team={scope.team}
	setting={data.notifications}
	locked={scope.archived}
/>
