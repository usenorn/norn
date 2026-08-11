<script lang="ts">
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleSlash from "@lucide/svelte/icons/circle-slash";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { checkStateLabels, checkStateTones, type CheckState } from "./checks";

	let { state, class: className = "" }: { state: CheckState; class?: string } = $props();

	const glyphs = {
		unproven: CircleDashed,
		proven: CircleCheck,
		failed: CircleX,
		waived: CircleSlash,
		gap: TriangleAlert,
	};

	const Glyph = $derived(glyphs[state]);
</script>

<Glyph
	class="size-icon-row shrink-0 {checkStateTones[state]} {className}"
	aria-hidden="true"
/>
<span class="sr-only">{checkStateLabels[state]}</span>
