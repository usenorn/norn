<script lang="ts">
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		reviewBodyMaxLength,
		draftsLine,
		submitLabel,
		verdictChoice,
		verdictLine,
		type ReviewVerdict,
	} from "./review";

	let {
		drafts,
		working,
		locked,
		onsubmit,
	}: {
		drafts: number;
		working: boolean;
		locked?: string;
		onsubmit: (verdict: ReviewVerdict, summary: string) => Promise<boolean>;
	} = $props();

	let open = $state(false);
	let verdict = $state<ReviewVerdict>("comment");
	let summary = $state("");

	const verdicts: ReviewVerdict[] = ["comment", "approve", "request_changes"];

	const empty = $derived(verdict !== "approve" && summary.trim() === "" && drafts === 0);

	async function send() {
		if (await onsubmit(verdict, summary.trim())) {
			open = false;
			summary = "";
			verdict = "comment";
		}
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} size="sm" disabled={working}>
				Finish your review
				{#if drafts > 0}
					<span class="rounded-xs bg-primary-foreground/20 px-1 font-mono text-2xs">{drafts}</span>
				{/if}
				<ChevronDown aria-hidden="true" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content align="end" size="body" class="flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-3">
		<Textarea
			bind:value={summary}
			rows={4}
			maxlength={reviewBodyMaxLength}
			disabled={working}
			aria-label="Summary of your review"
			placeholder="Leave a summary. It goes to the coding agent with your comments."
		/>

		<RadioGroup.Root bind:value={verdict} disabled={working} aria-label="What you decide">
			{#each verdicts as choice (choice)}
				<div class="flex items-start gap-2">
					<RadioGroup.Item
						id={`verdict-${choice}`}
						value={choice}
						class="mt-0.5"
						disabled={choice !== "comment" && locked !== undefined}
					/>
					<label for={`verdict-${choice}`} class="flex flex-col gap-0.5">
						<span class="text-sm leading-normal text-ink-900">{verdictChoice(choice)}</span>
						<span class="text-xs leading-normal text-muted-foreground">{verdictLine(choice)}</span>
					</label>
				</div>
			{/each}
		</RadioGroup.Root>

		{#if locked}
			<p class="text-2xs text-muted-foreground">{locked}</p>
		{/if}

		<div class="flex flex-wrap items-center justify-between gap-2">
			<span class="text-2xs text-muted-foreground">
				{draftsLine(drafts)}
			</span>
			<Button size="sm" disabled={working || empty} onclick={send}>
				{submitLabel(verdict, drafts)}
			</Button>
		</div>
	</Popover.Content>
</Popover.Root>
