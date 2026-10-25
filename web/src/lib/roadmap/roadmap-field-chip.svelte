<script lang="ts" module>
	export type RoadmapFieldTone = "shown" | "hidden" | "exposed" | "changed" | "concealed";
</script>

<script lang="ts">
	import Eye from "@lucide/svelte/icons/eye";
	import EyeOff from "@lucide/svelte/icons/eye-off";
	import PencilLine from "@lucide/svelte/icons/pencil-line";
	import Pill, { type PillTone } from "#lib/components/norn/pill.svelte";

	let { label, tone, note }: { label: string; tone: RoadmapFieldTone; note?: string } = $props();

	const pillTones: Record<RoadmapFieldTone, PillTone> = {
		shown: "success",
		hidden: "muted",
		exposed: "warning",
		changed: "active",
		concealed: "muted",
	};
</script>

<Pill tone={pillTones[tone]}>
	{#if tone === "changed"}
		<PencilLine aria-hidden="true" />
	{:else if tone === "hidden" || tone === "concealed"}
		<EyeOff aria-hidden="true" />
	{:else}
		<Eye aria-hidden="true" />
	{/if}
	<span>{label}</span>
	{#if note}
		<span class="sr-only">· {note}</span>
	{/if}
</Pill>
