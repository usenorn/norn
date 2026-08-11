<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { buttonVariants } from "$lib/components/ui/button/index.js";
	import CheckGlyph from "./check-glyph.svelte";
	import { blockingLine, type IssueCheck, type IssueCheckSummary } from "./checks";

	let {
		open,
		reference,
		stateName,
		blocking,
		summary,
		working,
		onconfirm,
		onclose,
	}: {
		open: boolean;
		reference: string;
		stateName: string;
		blocking: IssueCheck[];
		summary: IssueCheckSummary;
		working: boolean;
		onconfirm: () => void;
		onclose: () => void;
	} = $props();
</script>

<AlertDialog.Root {open} onOpenChange={(next) => !next && onclose()}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Finish {reference} without proving it?</AlertDialog.Title>
			<AlertDialog.Description>
				{blockingLine(summary)} Norn will move it to {stateName} and record that you decided to
				let it past, with your name on that entry.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<ul class="flex flex-col gap-1.5">
			{#each blocking as check (check.id)}
				<li class="flex min-w-0 items-start gap-2">
					<span class="mt-0.5">
						<CheckGlyph state={check.state ?? "unproven"} />
					</span>
					<span class="min-w-0 flex-1 text-sm leading-normal text-ink-900 text-pretty">
						{check.statement}
					</span>
				</li>
			{/each}
		</ul>

		<AlertDialog.Footer>
			<AlertDialog.Cancel variant="secondary" disabled={working} onclick={onclose}>
				Go back
			</AlertDialog.Cancel>
			<AlertDialog.Action
				class={buttonVariants({ variant: "default" })}
				disabled={working}
				onclick={onconfirm}
			>
				{working ? "Finishing…" : "Finish it anyway"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
