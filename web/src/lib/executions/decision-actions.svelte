<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";

	let {
		working,
		blocked,
		approveLabel,
		confirmPrompt,
		confirmLabel,
		requestLabel,
		feedbackLabel,
		feedbackPlaceholder,
		sendLabel,
		sendHint,
		feedbackMaxLength,
		onapprove,
		onrequest,
	}: {
		working: boolean;
		blocked?: string;
		approveLabel: string;
		confirmPrompt: string;
		confirmLabel: string;
		requestLabel: string;
		feedbackLabel: string;
		feedbackPlaceholder: string;
		sendLabel: string;
		sendHint: string;
		feedbackMaxLength: number;
		onapprove: () => void;
		onrequest: (feedback: string) => Promise<boolean>;
	} = $props();

	const blockedId = $props.id();

	let confirming = $state(false);
	let asking = $state(false);
	let feedback = $state("");

	async function send() {
		const said = feedback.trim();

		if (said === "") return;

		const sent = await onrequest(said);

		if (!sent) return;

		asking = false;
		feedback = "";
	}
</script>

<div class="flex min-w-0 flex-col gap-2">
	<div class="flex flex-wrap items-center gap-2">
		{#if confirming}
			<span class="text-xs text-muted-foreground">{confirmPrompt}</span>
			<Button
				size="sm"
				disabled={working}
				onclick={() => {
					confirming = false;
					onapprove();
				}}
			>
				{confirmLabel}
			</Button>
			<Button variant="ghost" size="sm" disabled={working} onclick={() => (confirming = false)}>
				Not yet
			</Button>
		{:else if !asking}
			<Button
				size="sm"
				disabled={working || blocked !== undefined}
				aria-describedby={blocked ? blockedId : undefined}
				onclick={() => (confirming = true)}
			>
				{approveLabel}
			</Button>
			<Button variant="secondary" size="sm" disabled={working} onclick={() => (asking = true)}>
				{requestLabel}
			</Button>
		{/if}
	</div>

	{#if blocked && !asking}
		<p id={blockedId} class="text-2xs text-muted-foreground">{blocked}</p>
	{/if}

	{#if asking}
		<div class="flex min-w-0 flex-col gap-2">
			<Textarea
				bind:value={feedback}
				rows={3}
				maxlength={feedbackMaxLength}
				disabled={working}
				aria-label={feedbackLabel}
				placeholder={feedbackPlaceholder}
			/>
			<div class="flex flex-wrap items-center gap-2">
				<Button size="sm" disabled={working || feedback.trim() === ""} onclick={send}>
					{sendLabel}
				</Button>
				<Button
					variant="ghost"
					size="sm"
					disabled={working}
					onclick={() => {
						asking = false;
						feedback = "";
					}}
				>
					Cancel
				</Button>
				<span class="text-2xs text-muted-foreground">{sendHint}</span>
			</div>
		</div>
	{/if}
</div>
