<script lang="ts">
	import { untrack } from "svelte";
	import AtSign from "@lucide/svelte/icons/at-sign";
	import Bot from "@lucide/svelte/icons/bot";
	import X from "@lucide/svelte/icons/x";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import AttachmentPicker from "$lib/attachments/attachment-picker.svelte";
	import UploadList from "$lib/attachments/upload-list.svelte";
	import { attachmentMarkdown } from "$lib/attachments/attachments";
	import { settled, type UploadTask } from "$lib/attachments/upload";
	import type { MentionTarget } from "$lib/comments/comments";
	import type { Member } from "$lib/issues/members";
	import type { Team } from "$lib/team/teams";

	type Candidate = { key: string; name: string; agent?: boolean; target: MentionTarget };

	let {
		members,
		teams,
		working = false,
		body = "",
		placeholder = "Leave a comment",
		submitLabel = "Comment",
		onsubmit,
		oncancel,
		uploads,
		onfiles,
		oncancelupload,
		onretryupload,
		ondismissupload,
	}: {
		members: Member[];
		teams: Team[];
		working?: boolean;
		body?: string;
		placeholder?: string;
		submitLabel?: string;
		onsubmit: (body: string, mentions: MentionTarget[], attachmentIds: string[]) => void;
		oncancel?: () => void;
		uploads?: UploadTask[];
		onfiles?: (files: File[]) => void;
		oncancelupload?: (id: string) => void;
		onretryupload?: (id: string) => void;
		ondismissupload?: (id: string) => void;
	} = $props();

	const id = $props.id();

	let draft = $state(untrack(() => body));
	let chosen = $state.raw<Candidate[]>([]);
	let field = $state<HTMLTextAreaElement | null>(null);
	let dropping = $state(false);

	const candidates = $derived<Candidate[]>([
		...members
			.filter((member) => Boolean(member.displayName))
			.map((member) => ({
				key: `account:${member.accountId}`,
				name: member.displayName ?? "",
				agent: member.kind === "agent",
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
	const inFlight = $derived((uploads ?? []).some((task) => !settled(task)));
	const attached = $derived(
		(uploads ?? []).filter((task) => task.state === "done" && task.attachment)
	);

	const spliced = new Set<string>();

	$effect(() => {
		for (const task of uploads ?? []) {
			if (task.state !== "done" || !task.attachment || spliced.has(task.id)) continue;

			spliced.add(task.id);
			insert(attachmentMarkdown(task.attachment));
		}
	});

	function insert(text: string) {
		const at = field?.selectionStart ?? draft.length;
		const spaced = draft === "" || draft.endsWith("\n") ? text : `\n${text}`;

		draft = draft.slice(0, at) + spaced + draft.slice(at);

		if (field) {
			const caret = at + spaced.length;

			field.focus();
			requestAnimationFrame(() => field?.setSelectionRange(caret, caret));
		}
	}

	function take(files: File[]) {
		if (files.length === 0 || !onfiles) return;

		onfiles(files);
	}

	function dropped(event: DragEvent) {
		dropping = false;

		take(Array.from(event.dataTransfer?.files ?? []));
	}

	function pasted(event: ClipboardEvent) {
		const files = Array.from(event.clipboardData?.files ?? []);

		if (files.length === 0) return;

		event.preventDefault();
		take(files);
	}

	function mention(candidate: Candidate) {
		chosen = [...chosen, candidate];
		draft = draft === "" ? `@${candidate.name} ` : `${draft} @${candidate.name} `;
	}

	function drop(key: string) {
		chosen = chosen.filter((candidate) => candidate.key !== key);
	}

	function send() {
		if (empty || working || inFlight) return;

		const mentions = chosen.map((candidate) => candidate.target);
		const attachmentIds = attached.map((task) => task.attachment?.id ?? "").filter(Boolean);

		onsubmit(draft.trim(), mentions, attachmentIds);

		if (!oncancel) {
			draft = "";
			chosen = [];
		}
	}
</script>

<div
	class="flex flex-col gap-2 rounded-lg {dropping ? 'border border-dashed border-ring p-2' : ''}"
	role="group"
	ondragover={(event) => {
		if (!onfiles) return;
		event.preventDefault();
		dropping = true;
	}}
	ondragleave={() => (dropping = false)}
	ondrop={(event) => {
		if (!onfiles) return;
		event.preventDefault();
		dropped(event);
	}}
>
	<Label for="comment-{id}" class="sr-only">{placeholder}</Label>
	<Textarea
		bind:ref={field}
		id="comment-{id}"
		bind:value={draft}
		{placeholder}
		rows={4}
		disabled={working}
		class="font-mono text-sm"
		onpaste={pasted}
	/>
	<p class="text-xs text-muted-foreground">
		Markdown. Stored exactly as you type it.{onfiles ? " Drop or paste a file to attach it." : ""}
	</p>

	{#if uploads && uploads.length > 0}
		<UploadList
			{uploads}
			oncancel={(taskId) => oncancelupload?.(taskId)}
			onretry={(taskId) => onretryupload?.(taskId)}
			ondismiss={(taskId) => ondismissupload?.(taskId)}
		/>
	{/if}

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
		<Button size="sm" disabled={working || empty || inFlight} onclick={send}>
			{working ? "Working" : inFlight ? "Uploading" : submitLabel}
		</Button>

		{#if onfiles}
			<AttachmentPicker disabled={working} onfiles={take} />
		{/if}

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
						<span class="flex items-center gap-1.5">
							{#if candidate.agent}
								<Bot class="size-3.5 text-muted-foreground" aria-label="An agent" />
							{/if}
							{candidate.name}
						</span>
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		{#if oncancel}
			<Button variant="ghost" size="sm" disabled={working} onclick={oncancel}>Cancel</Button>
		{/if}
	</div>
</div>
