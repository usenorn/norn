<script lang="ts">
	import { untrack } from "svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { reviewBodyMaxLength } from "./review";

	type Choice = { label: string; publish: boolean };

	let {
		label,
		initial = "",
		placeholder,
		primary,
		secondary,
		working,
		onsubmit,
		oncancel,
	}: {
		label: string;
		initial?: string;
		placeholder: string;
		primary: Choice;
		secondary?: Choice;
		working: boolean;
		onsubmit: (body: string, publish: boolean) => Promise<boolean>;
		oncancel: () => void;
	} = $props();

	let body = $state(untrack(() => initial));
	let field = $state<HTMLTextAreaElement | null>(null);

	$effect(() => {
		field?.focus();
	});

	async function send(choice: Choice) {
		const said = body.trim();

		if (said === "") return;

		if (await onsubmit(said, choice.publish)) body = "";
	}

	function keyed(event: KeyboardEvent) {
		if (event.key === "Escape") {
			event.preventDefault();
			oncancel();
		}

		if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
			event.preventDefault();
			void send(primary);
		}
	}
</script>

<div class="flex min-w-0 flex-col gap-2 font-sans">
	<Textarea
		bind:ref={field}
		bind:value={body}
		rows={3}
		maxlength={reviewBodyMaxLength}
		disabled={working}
		aria-label={label}
		{placeholder}
		onkeydown={keyed}
	/>
	<div class="flex flex-wrap items-center gap-2">
		<Button size="sm" disabled={working || body.trim() === ""} onclick={() => send(primary)}>
			{primary.label}
		</Button>
		{#if secondary}
			<Button
				variant="secondary"
				size="sm"
				disabled={working || body.trim() === ""}
				onclick={() => send(secondary)}
			>
				{secondary.label}
			</Button>
		{/if}
		<Button variant="ghost" size="sm" disabled={working} onclick={oncancel}>Cancel</Button>
		<span class="text-2xs text-muted-foreground">Markdown works. Ctrl or ⌘ with Enter sends it.</span>
	</div>
</div>
