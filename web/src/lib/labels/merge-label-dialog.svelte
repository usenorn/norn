<script lang="ts">
	import ArrowRight from "@lucide/svelte/icons/arrow-right";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { issueCount, mergeTargets, usageOf, type Label, type LabelUsage } from "./labels";

	let {
		open = $bindable(false),
		source,
		labels,
		usage,
		merging,
		onconfirm,
	}: {
		open?: boolean;
		source: Label | null;
		labels: Label[];
		usage: LabelUsage;
		merging: boolean;
		onconfirm: (targetId: string) => void;
	} = $props();

	let targetId = $state("");

	const targets = $derived(source ? mergeTargets(source, labels) : []);
	const target = $derived(targets.find((candidate) => candidate.id === targetId) ?? null);

	$effect(() => {
		if (!open) targetId = "";
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-120">
		<Dialog.Header>
			<Eyebrow>Merge labels</Eyebrow>
			<Dialog.Title>Two labels for one idea</Dialog.Title>
			<Dialog.Description class="sr-only">
				Move every issue carrying one label onto another and remove the first.
			</Dialog.Description>
		</Dialog.Header>

		{#if source}
			{#if targets.length === 0}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Nothing can absorb {source.name}. A target has to be in the same group and cover this
					label's scope, so a workspace label cannot merge into a team one.
				</p>
			{:else}
				<div class="flex flex-col gap-4">
					<div class="flex flex-col gap-3 sm:flex-row sm:items-start">
						<div class="flex min-w-0 flex-1 flex-col gap-1.5">
							<Eyebrow>Disappears</Eyebrow>
							<Tag name={source.name} color={source.color} />
							<span class="font-mono text-2xs text-muted-foreground">
								{issueCount(usageOf(usage, source))}
							</span>
						</div>

						<span
							class="shrink-0 self-center text-muted-foreground sm:self-start sm:pt-6"
							aria-hidden="true"
						>
							<ArrowRight class="size-3.5" />
						</span>

						<div class="flex min-w-0 flex-1 flex-col gap-1.5">
							<Eyebrow>Survives</Eyebrow>
							{#if target}
								<Tag name={target.name} color={target.color} />
								<span class="font-mono text-2xs text-muted-foreground">
									{issueCount(usageOf(usage, target))}
								</span>
							{:else}
								<span class="text-sm text-muted-foreground">Choose a label below.</span>
							{/if}
						</div>
					</div>

					<span class="h-px bg-line-subtle" aria-hidden="true"></span>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Every issue carrying {source.name} moves to the label you choose, and {source.name} is
						removed. Issues already carrying both keep one. The merge cannot be undone.
					</p>

					<div class="max-w-60">
						<Select.Root
							type="single"
							value={targetId}
							disabled={merging}
							onValueChange={(value) => (targetId = value)}
						>
							<Select.Trigger aria-label="Merge into">
								{target?.name ?? "Choose a label"}
							</Select.Trigger>
							<Select.Content>
								{#each targets as candidate (candidate.id)}
									<Select.Item value={candidate.id} label={candidate.name}>
										{candidate.name}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
				</div>
			{/if}
		{/if}

		<Dialog.Footer>
			<Button variant="ghost" disabled={merging} onclick={() => (open = false)}>Cancel</Button>
			{#if targets.length > 0}
				<Button disabled={merging || !targetId} onclick={() => onconfirm(targetId)}>
					{merging ? "Merging" : "Merge labels"}
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
