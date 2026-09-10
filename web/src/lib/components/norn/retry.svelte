<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { keys } from "$lib/api/keys";
	import { Button } from "$lib/components/ui/button/index.js";

	let {
		label = "Try again",
		variant = "secondary",
	}: { label?: string; variant?: "secondary" | "outline" } = $props();

	let asking = $state(false);

	async function again() {
		asking = true;

		try {
			await invalidate(keys.page(page.route.id));
		} finally {
			asking = false;
		}
	}
</script>

<Button {variant} size="sm" onclick={again} disabled={asking}>
	{asking ? "Trying" : label}
</Button>
