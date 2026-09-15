<script lang="ts">
	import TemplateSettings from "$lib/issues/template-settings.svelte";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const scope = useTeamSettings();
</script>

<TemplateSettings
	workspaceId={data.workspace.id}
	workspace={data.workspace.slug}
	teamId={scope.team.id}
	teamName={scope.team.name}
	locked={scope.archived || scope.readOnly}
/>
