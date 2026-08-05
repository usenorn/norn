<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";

	let { shown, off = false }: { shown: boolean; off?: boolean } = $props();

	let refreshing = $state(false);

	async function refresh() {
		refreshing = true;

		try {
			await invalidateAll();
		} finally {
			refreshing = false;
		}
	}
</script>

{#if shown}
	<div class="px-4 pt-3">
		<Alert.Root variant="warning">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>{off ? "Live updates are off" : "Not receiving updates"}</Alert.Title>
			<Alert.Description>
				What you are looking at may be out of date. Everything still works &mdash; changes you
				make are saved as normal.
			</Alert.Description>
			<Alert.Action>
				<Button variant="secondary" size="sm" disabled={refreshing} onclick={refresh}>
					{refreshing ? "Refreshing…" : "Refresh"}
				</Button>
			</Alert.Action>
		</Alert.Root>
	</div>
{/if}
