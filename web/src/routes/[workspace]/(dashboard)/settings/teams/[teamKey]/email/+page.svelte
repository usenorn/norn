<script lang="ts">
	import { page } from "$app/state";
	import TeamIntake from "$lib/team/team-intake.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { teamEmailPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();

	const preview = $derived(
		import.meta.env.DEV
			? teamEmailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
</script>

<TeamIntake
	workspace={data.workspace}
	team={scope.team}
	setting={preview?.intake ?? data.intake}
	locked={scope.archived || scope.readOnly}
/>
