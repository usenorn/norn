<script lang="ts">
	import { page } from "$app/state";
	import WorkflowStates from "$lib/components/norn/workflow-states.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { teamStatesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();

	const preview = $derived(
		import.meta.env.DEV
			? teamStatesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
</script>

<WorkflowStates
	workspaceId={data.workspace.id}
	team={scope.team}
	list={preview?.states ?? data.states}
	locked={scope.archived || scope.readOnly}
/>
