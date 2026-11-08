<script lang="ts">
	import type { Snippet } from "svelte";
	import * as Tooltip from "#lib/components/ui/tooltip/index.js";
	import type { PersonDetail } from "#lib/workspace/members.js";
	import PersonDetails from "./person-details.svelte";

	let { person, children }: { person: PersonDetail; children: Snippet } = $props();

	let open = $state(false);
	let touched = false;

	function notePointer(event: PointerEvent) {
		touched = event.pointerType === "touch";
	}

	function toggleOnTap() {
		if (touched) open = !open;

		touched = false;
	}
</script>

<Tooltip.Root bind:open disableCloseOnTriggerClick ignoreNonKeyboardFocus>
	<Tooltip.Trigger
		aria-label={person.name}
		onpointerdown={notePointer}
		onclick={toggleOnTap}
		class="relative rounded-full focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
	>
		{@render children()}
	</Tooltip.Trigger>
	<Tooltip.Content>
		<PersonDetails {person} />
	</Tooltip.Content>
</Tooltip.Root>
