<script lang="ts">
	import { page } from "$app/state";
	import CycleCadence from "$lib/team/cycle-cadence.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { teamCyclesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();

	const preview = $derived(
		import.meta.env.DEV
			? teamCyclesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
</script>

<CycleCadence
	workspace={data.workspace}
	team={scope.team}
	setting={preview?.cadence ?? data.cadence}
	locked={scope.archived || scope.readOnly}
/>
