<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		canApprove,
		canRequestChanges,
		feedbackMaxLength,
		type Execution,
	} from "./executions";

	let {
		execution,
		working,
		onapprove,
		onrequestchanges,
	}: {
		execution: Execution;
		working: boolean;
		onapprove: () => void;
		onrequestchanges: (feedback: string) => void;
	} = $props();

	let confirming = $state(false);
	let asking = $state(false);
	let feedback = $state("");

	function send() {
		const said = feedback.trim();

		if (said === "") return;

		asking = false;
		feedback = "";
		onrequestchanges(said);
	}
</script>

{#if canApprove(execution) || canRequestChanges(execution)}
	<div class="flex min-w-0 flex-col gap-2">
		<div class="flex flex-wrap items-center gap-2">
			{#if canApprove(execution)}
				{#if confirming}
					<span class="text-xs text-muted-foreground">Accept this work?</span>
					<Button
						size="sm"
						disabled={working}
						onclick={() => {
							confirming = false;
							onapprove();
						}}
					>
						Accept it
					</Button>
					<Button variant="ghost" size="sm" disabled={working} onclick={() => (confirming = false)}>
						Not yet
					</Button>
				{:else}
					<Button
						size="sm"
						disabled={working}
						onclick={() => {
							asking = false;
							confirming = true;
						}}
					>
						Approve
					</Button>
				{/if}
			{/if}

			{#if canRequestChanges(execution) && !confirming && !asking}
				<Button
					variant="secondary"
					size="sm"
					disabled={working}
					onclick={() => (asking = true)}
				>
					Request changes
				</Button>
			{/if}
		</div>

		{#if asking}
			<div class="flex min-w-0 flex-col gap-2">
				<Textarea
					bind:value={feedback}
					rows={3}
					maxlength={feedbackMaxLength}
					disabled={working}
					aria-label="What should change"
					placeholder="Say what should change. It is handed to the coding agent word for word."
				/>
				<div class="flex flex-wrap items-center gap-2">
					<Button size="sm" disabled={working || feedback.trim() === ""} onclick={send}>
						Send it back
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
					<span class="text-2xs text-muted-foreground">
						The same run carries on, on the same branches, with the same preview addresses.
					</span>
				</div>
			</div>
		{/if}
	</div>
{/if}
