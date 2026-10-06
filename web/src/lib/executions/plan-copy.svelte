<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import Copy from "@lucide/svelte/icons/copy";
	import { copyText } from "$lib/clipboard";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { copiedPlanLine } from "./plan-review";

	let { text, blocked }: { text: string; blocked?: string } = $props();

	let outcome = $state<"idle" | "copied" | "refused">("idle");

	const reasonId = $props.id();

	$effect(() => {
		void text;
		outcome = "idle";
	});

	async function copy() {
		outcome = (await copyText(text, copiedPlanLine)) ? "copied" : "refused";
	}
</script>

<div class="flex min-w-0 flex-col gap-2">
	<Button
		variant="secondary"
		size="sm"
		class="w-max"
		disabled={blocked !== undefined}
		aria-describedby={blocked !== undefined ? reasonId : undefined}
		onclick={copy}
	>
		{#if outcome === "copied"}
			<Check aria-hidden="true" />
			Copied
		{:else}
			<Copy aria-hidden="true" />
			Copy for review
		{/if}
	</Button>

	{#if blocked !== undefined}
		<p id={reasonId} class="text-2xs text-muted-foreground">{blocked}</p>
	{/if}

	{#if outcome === "refused"}
		<Textarea
			readonly
			value={text}
			aria-label="Plan for review, select it all and copy"
			class="max-h-96 overflow-auto font-mono text-xs"
			onfocus={(event) => event.currentTarget.select()}
		/>
	{/if}
</div>
