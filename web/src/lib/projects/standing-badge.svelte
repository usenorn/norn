<script lang="ts">
	import Pill, { type PillTone } from "#lib/components/norn/pill.svelte";
	import { standingLabels, type ProjectStanding } from "./projects";

	let { standing, class: className }: { standing: ProjectStanding; class?: string } = $props();

	const tones: Record<ProjectStanding, PillTone> = {
		on_track: "success",
		at_risk: "warning",
		completed: "success",
		cancelled: "muted",
	};
</script>

<Pill tone={tones[standing]} class={className}>{standingLabels[standing]}</Pill>
