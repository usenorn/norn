<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import type { SourceControlVerdict } from "./source-control";

	let { verdict }: { verdict: SourceControlVerdict } = $props();

	const icon = $derived(
		verdict.tone === "success" ? CircleCheck : verdict.tone === "muted" ? CircleAlert : TriangleAlert,
	);

	const Icon = $derived(icon);
</script>

<Alert.Root
	variant={verdict.tone}
	role={verdict.tone === "destructive" ? "alert" : "status"}
	aria-live={verdict.tone === "destructive" ? "assertive" : "polite"}
>
	<Icon class="size-icon-row shrink-0" aria-hidden="true" />
	<Alert.Title>{verdict.title}</Alert.Title>
	<Alert.Description>
		<span class="text-pretty">{verdict.detail}</span>
		{#if verdict.action}
			<Button variant="secondary" href={verdict.action.href} class="mt-3 self-start">
				{verdict.action.label}
			</Button>
		{/if}
	</Alert.Description>
</Alert.Root>
