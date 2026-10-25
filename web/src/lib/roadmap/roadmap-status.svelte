<script lang="ts">
	import Pill, { type PillTone } from "#lib/components/norn/pill.svelte";
	import { statusLabels, type RoadmapPublicStatus } from "./public-roadmap";

	let { status }: { status: RoadmapPublicStatus } = $props();

	const tones: Record<RoadmapPublicStatus, PillTone> = {
		planned: "muted",
		in_progress: "active",
		paused: "warning",
		completed: "success",
		cancelled: "muted",
	};
</script>

<Pill tone={tones[status]}>{statusLabels[status]}</Pill>
