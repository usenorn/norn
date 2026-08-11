<script lang="ts">
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import Ellipsis from "@lucide/svelte/icons/ellipsis";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import CheckGlyph from "./check-glyph.svelte";
	import EvidenceList from "./evidence-list.svelte";
	import {
		awaitingLabels,
		checkStateLabels,
		checkStateTones,
		evidenceAge,
		methodLabels,
		type EvidencePanel,
		type IssueCheck,
	} from "./checks";

	let {
		check,
		slug,
		timezone,
		canManage,
		working,
		expanded,
		evidence,
		gapReference,
		ontoggle,
		onfile,
		onwaive,
		ongap,
		onremove,
	}: {
		check: IssueCheck;
		slug: string;
		timezone: string;
		canManage: boolean;
		working: boolean;
		expanded: boolean;
		evidence: EvidencePanel;
		gapReference: string | undefined;
		ontoggle: () => void;
		onfile: () => void;
		onwaive: () => void;
		ongap: () => void;
		onremove: () => void;
	} = $props();

	const state = $derived(check.state ?? "unproven");
	const pending = $derived(check.approval === "pending");
	const declined = $derived(check.approval === "declined");
	const settled = $derived(check.resolution !== "none");
	const window = $derived(evidenceAge(check));
	const count = $derived(check.evidenceCount ?? 0);
</script>

<li class="flex min-w-0 flex-col gap-1.5 py-2">
	<div class="flex min-w-0 items-start gap-2">
		<span class="mt-0.5">
			<CheckGlyph {state} />
		</span>

		<div class="flex min-w-0 flex-1 flex-col gap-1">
			<div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
				<span class="min-w-0 flex-1 text-sm leading-normal text-ink-900 text-pretty">
					{check.statement}
				</span>
				<span class="text-xs {checkStateTones[state]}">{checkStateLabels[state]}</span>
			</div>

			<div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
				<span class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
					{methodLabels[check.method]}
				</span>
				<span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">{check.proof}</span>
			</div>

			{#if check.addedAfterDelegation || check.authorKind === "agent"}
				<div class="flex flex-wrap items-center gap-1.5">
					{#if check.addedAfterDelegation}
						<Tag name="Added late" />
					{/if}
					{#if check.authorKind === "agent"}
						<Tag name="Agent wrote this" />
					{/if}
				</div>
			{/if}

			{#if declined}
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					Declined, so it takes no evidence and stands in nobody's way.
				</p>
			{:else if pending}
				<p class="text-xs leading-normal text-warning text-pretty">
					Waiting for a person. Until somebody approves it, this changes nothing about whether
					the issue can be finished. An agent proposes its criteria as one set, decided together
					on the
					<a
						href={workspacePath(slug, "/agents/approvals")}
						class="underline underline-offset-2 hover:text-ink-900"
					>
						approvals screen
					</a>.
				</p>
			{:else if settled}
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					{check.resolution === "waived" ? "Waived" : "Recorded as a gap"}{check.resolutionReason
						? `: ${check.resolutionReason}`
						: "."}
					{#if check.resolution === "gap" && gapReference}
						<a
							href={workspacePath(slug, `/issues/${gapReference}`)}
							class="font-mono underline underline-offset-2 hover:text-ink-900"
						>
							{gapReference}
						</a>
						carries it.
					{/if}
				</p>
			{:else if check.awaiting}
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					{awaitingLabels[check.awaiting]}
				</p>
			{:else if state === "proven" && window}
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					Proven. This proof counts for {window} from when it was filed.
				</p>
			{/if}
		</div>

		{#if canManage}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="ghost"
							size="icon-xs"
							disabled={working}
							aria-label="What to do with this criterion"
						>
							<Ellipsis aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.Item disabled={declined || settled} onSelect={onfile}>
						File evidence
					</DropdownMenu.Item>
					<DropdownMenu.Item disabled={settled} onSelect={onwaive}>
						Waive it, with a reason
					</DropdownMenu.Item>
					<DropdownMenu.Item disabled={settled} onSelect={ongap}>
						Record a gap and file the issue
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					<DropdownMenu.Item variant="destructive" onSelect={onremove}>
						Remove it from what done means
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{/if}
	</div>

	<button
		type="button"
		onclick={ontoggle}
		aria-expanded={expanded}
		class="-ml-1 flex w-max items-center gap-1 rounded-sm py-0.5 pr-1.5 pl-1 text-xs text-muted-foreground motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
	>
		<ChevronRight
			class="size-3 motion-control {expanded ? 'rotate-90' : ''}"
			aria-hidden="true"
		/>
		{count === 0 ? "No evidence yet" : count === 1 ? "1 record" : `${count} records`}
	</button>

	{#if expanded}
		<div class="min-w-0 pl-4">
			<EvidenceList panel={evidence} {timezone} />
		</div>
	{/if}
</li>
