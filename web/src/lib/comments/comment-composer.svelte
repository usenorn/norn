<script lang="ts">
	import { untrack } from "svelte";
	import AtSign from "@lucide/svelte/icons/at-sign";
	import X from "@lucide/svelte/icons/x";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import type { MentionTarget } from "$lib/comments/comments";
	import type { Member } from "$lib/issues/members";
	import type { Team } from "$lib/team/teams";

	type Candidate = { key: string; name: string; target: MentionTarget };

	let {
		members,
		teams,
		working = false,
		body = "",
		placeholder = "Leave a comment",
		submitLabel = "Comment",
		onsubmit,
		oncancel,
	}: {
		members: Member[];
		teams: Team[];
		working?: boolean;
		body?: string;
		placeholder?: string;
		submitLabel?: string;
		onsubmit: (body: string, mentions: MentionTarget[]) => void;
		oncancel?: () => void;
	} = $props();

	const id = $props.id();

	let draft = $state(untrack(() => body));
	let chosen = $state.raw<Candidate[]>([]);

	const candidates = $derived<Candidate[]>([
		...members
			.filter((member) => Boolean(member.displayName))
			.map((member) => ({
				key: `account:${member.accountId}`,
				name: member.displayName ?? "",
				target: { kind: "account" as const, accountId: member.accountId },
			})),
		...teams.map((team) => ({
			key: `team:${team.id}`,
			name: team.name,
			target: { kind: "team" as const, teamId: team.id },
		})),
	]);

	const picked = $derived(new Set(chosen.map((candidate) => candidate.key)));
	const available = $derived(candidates.filter((candidate) => !picked.has(candidate.key)));
	const empty = $derived(draft.trim() === "");

	function mention(candidate: Candidate) {
		chosen = [...chosen, candidate];
		draft = draft === "" ? `@${candidate.name} ` : `${draft} @${candidate.name} `;
	}

	function drop(key: string) {
		chosen = chosen.filter((candidate) => candidate.key !== key);
	}

	function send() {
		if (empty || working) return;

		const mentions = chosen.map((candidate) => candidate.target);

		onsubmit(draft.trim(), mentions);

		if (!oncancel) {
			draft = "";
			chosen = [];
		}
	}
</script>

<div class="flex flex-col gap-2">
	<Label for="comment-{id}" class="sr-only">{placeholder}</Label>
	<Textarea
		id="comment-{id}"
		bind:value={draft}
		{placeholder}
		rows={4}
		disabled={working}
		class="font-mono text-sm"
	/>
	<p class="text-xs text-muted-foreground">Markdown. Stored exactly as you type it.</p>

	{#if chosen.length > 0}
		<ul class="flex flex-wrap gap-1">
			{#each chosen as candidate (candidate.key)}
				<li>
					<Button
						variant="secondary"
						size="sm"
						disabled={working}
						aria-label="Stop mentioning {candidate.name}"
						onclick={() => drop(candidate.key)}
					>
						{candidate.name}
						<X aria-hidden="true" />
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="flex flex-wrap items-center gap-2">
		<Button size="sm" disabled={working || empty} onclick={send}>
			{working ? "Working" : submitLabel}
		</Button>

		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						size="sm"
						disabled={working || available.length === 0}
					>
						<AtSign aria-hidden="true" />
						Mention
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="max-h-64 overflow-y-auto">
				{#each available as candidate (candidate.key)}
					<DropdownMenu.Item onSelect={() => mention(candidate)}>
						{candidate.name}
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		{#if oncancel}
			<Button variant="ghost" size="sm" disabled={working} onclick={oncancel}>Cancel</Button>
		{/if}
	</div>
</div>
