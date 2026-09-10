<script lang="ts">
	import { Editor } from "@tiptap/core";
	import type { EditorView } from "@tiptap/pm/view";
	import { markdownProse } from "$lib/issues/markdown";
	import {
		asDocument,
		emptyDocument,
		sameDocument,
		withParagraph,
		type Document,
		type DocumentNode,
	} from "$lib/editor/document";
	import { editorExtensions } from "$lib/editor/schema";
	import { completing, type SuggestionAnchor, type SuggestionSession } from "$lib/editor/suggest";
	import { insertIssueRef, insertLink, insertMention, runBlock } from "$lib/editor/blocks";
	import { matchingCommands, type SlashCommand } from "$lib/editor/slash";
	import { findIssues, findMentions, type Suggestion } from "$lib/editor/search";
	import { composing } from "$lib/editor/keys";
	import { dropUpload, placeUpload, previewOf, uploadPosition } from "$lib/editor/uploads";
	import SuggestionPopup, { type PopupRow } from "$lib/editor/suggestion-popup.svelte";
	import Toolbar from "$lib/editor/toolbar.svelte";

	let {
		document = $bindable(emptyDocument),
		workspaceId,
		workspace,
		placeholder = "Write something…",
		disabled = false,
		autofocus = false,
		toolbar = true,
		minHeight = "min-h-16",
		id,
		label = "Description",
		onfiles,
		onmetaenter,
		onsubissue,
		class: className = "",
		...rest
	}: {
		document?: Document;
		workspaceId: string;
		workspace: string;
		placeholder?: string;
		disabled?: boolean;
		autofocus?: boolean;
		toolbar?: boolean;
		minHeight?: string;
		id?: string;
		label?: string;
		onfiles?: (files: File[]) => string[] | void;
		onmetaenter?: () => boolean;
		onsubissue?: (text: string) => void;
		class?: string;
		"aria-describedby"?: string;
		"aria-invalid"?: boolean | "true" | "false";
	} = $props();

	type Popup = {
		kind: "slash" | "mention" | "issue";
		rows: PopupRow[];
		chosen: (Suggestion | SlashCommand)[];
		index: number;
		anchor: SuggestionAnchor;
		state: "ready" | "loading" | "typing" | "empty" | "failed";
		take: (chosen: unknown) => void;
	};

	let editor = $state.raw<Editor | null>(null);
	let popup = $state.raw<Popup | null>(null);
	let revision = $state(0);
	let filing = $state.raw<HTMLInputElement | null>(null);
	let held: Document = emptyDocument;
	let asked = 0;

	const listId = $props.id();
	const optionId = (at: number) => `${listId}-option-${at}`;

	function place(session: SuggestionSession) {
		return { anchor: session.anchor };
	}

	function closing(kind: Popup["kind"]) {
		if (popup?.kind === kind) popup = null;
	}

	function slashRows(commands: SlashCommand[]): PopupRow[] {
		return commands.map((command) => ({
			key: command.key,
			label: command.label,
			hint: command.hint,
			group: command.group,
			shortcut: command.shortcut,
		}));
	}

	function suggestionRows(groups: { label: string; items: Suggestion[] }[]): {
		rows: PopupRow[];
		chosen: Suggestion[];
	} {
		const rows: PopupRow[] = [];
		const chosen: Suggestion[] = [];

		for (const group of groups) {
			for (const item of group.items) {
				rows.push({
					key: item.key,
					label: item.label,
					hint: item.hint,
					group: group.label,
					icon: item.kind === "agent" ? "agent" : item.kind,
				});
				chosen.push(item);
			}
		}

		return { rows, chosen };
	}

	function openSlash(session: SuggestionSession) {
		const commands = matchingCommands(session.query);

		popup = {
			kind: "slash",
			rows: slashRows(commands),
			chosen: commands,
			index: 0,
			state: "ready",
			take: session.take,
			...place(session),
		};
	}

	async function openSearch(kind: "mention" | "issue", session: SuggestionSession) {
		const mine = (asked += 1);
		const standing = popup?.kind === kind ? popup.rows[popup.index]?.key : undefined;

		popup = {
			kind,
			rows: popup?.kind === kind ? popup.rows : [],
			chosen: popup?.kind === kind ? popup.chosen : [],
			index: popup?.kind === kind ? popup.index : 0,
			state: "loading",
			take: session.take,
			...place(session),
		};

		const outcome =
			kind === "mention"
				? await findMentions(workspaceId, workspace, session.query)
				: await findIssues(workspaceId, workspace, session.query);

		if (mine !== asked || popup?.kind !== kind) return;

		if (outcome.state !== "ready") {
			popup = { ...popup, rows: [], chosen: [], index: 0, state: outcome.state, take: session.take };

			return;
		}

		const { rows, chosen } = suggestionRows(outcome.groups);
		const kept = rows.findIndex((row) => row.key === standing);

		popup = {
			...popup,
			rows,
			chosen,
			index: kept >= 0 ? kept : 0,
			state: "ready",
			take: session.take,
		};
	}

	function accept(at: number) {
		const open = popup;
		const within = editor;

		if (!open || !within || at < 0 || at >= open.chosen.length) return;

		const chosen = open.chosen[at];

		if (open.kind === "slash") {
			open.take({ command: chosen as SlashCommand });

			return;
		}

		open.take({ suggestion: chosen as Suggestion });
	}

	function steer(event: KeyboardEvent): boolean {
		const open = popup;

		if (!open || composing(event)) return false;

		if (event.key === "ArrowDown" || event.key === "ArrowUp") {
			if (open.rows.length === 0) return true;

			const step = event.key === "ArrowDown" ? 1 : -1;

			popup = { ...open, index: (open.index + step + open.rows.length) % open.rows.length };

			return true;
		}

		if (event.key === "Enter" || event.key === "Tab") {
			if (open.rows.length === 0) return open.state === "loading";

			accept(open.index);

			return true;
		}

		if (event.key === "Escape") {
			popup = null;
			event.stopPropagation();

			return true;
		}

		return false;
	}

	function runSlash(within: Editor, range: { from: number; to: number }, command: SlashCommand) {
		if (command.key === "image" || command.key === "file") {
			within.chain().focus().deleteRange(range).run();
			filing?.click();

			return;
		}

		if (command.key === "issue" || command.key === "mention" || command.key === "project") {
			const char = command.key === "issue" ? "#" : "@";

			within.chain().focus().deleteRange(range).insertContent(char).run();

			return;
		}

		if (command.key === "subissue") {
			const { from, to } = within.state.selection;
			const selected = within.state.doc.textBetween(from, to, " ").trim();

			within.chain().focus().deleteRange(range).run();

			onsubissue?.(selected === "" ? within.state.selection.$from.parent.textContent.trim() : selected);

			return;
		}

		runBlock(within, range, command.key);
	}

	function chose(within: Editor, range: { from: number; to: number }, suggestion: Suggestion) {
		if (suggestion.kind === "issue") {
			insertIssueRef(within, range, {
				issueId: suggestion.id,
				reference: suggestion.reference,
				title: suggestion.label,
				href: suggestion.href,
			});

			return;
		}

		if (suggestion.kind === "project") {
			insertLink(within, range, suggestion.label, suggestion.href);

			return;
		}

		insertMention(within, range, {
			id: suggestion.id,
			label: suggestion.label,
			kind: suggestion.kind === "team" ? "team" : "account",
		});
	}

	function taken(props: { editor: Editor; range: { from: number; to: number }; props: unknown }) {
		const chosen = props.props as { command?: SlashCommand; suggestion?: Suggestion };

		if (chosen.command) {
			runSlash(props.editor, props.range, chosen.command);

			return;
		}

		if (chosen.suggestion) chose(props.editor, props.range, chosen.suggestion);
	}

	function fileList(list: FileList | null | undefined): File[] {
		return Array.from(list ?? []);
	}

	function take(view: EditorView, dropped: File[], pos: number): boolean {
		if (dropped.length === 0 || !onfiles) return false;

		const ids = onfiles(dropped) ?? [];

		ids.forEach((taskId, at) => {
			const file = dropped[at];

			placeUpload(view, { id: taskId, name: file.name, preview: previewOf(file) }, pos);
		});

		return ids.length > 0;
	}

	export function settle(taskId: string, content: DocumentNode): boolean {
		const within = editor;

		if (!within) return false;

		const pos = uploadPosition(within.state, taskId);

		if (pos === null) return false;

		dropUpload(within.view, taskId);
		within.commands.insertContentAt(pos, content);

		return true;
	}

	export function abandon(taskId: string) {
		if (editor) dropUpload(editor.view, taskId);
	}

	export function focus() {
		editor?.commands.focus("end");
	}

	export function pending(taskId: string): boolean {
		const within = editor;

		return within ? uploadPosition(within.state, taskId) !== null : false;
	}

	function build(element: HTMLElement): Editor {
		return new Editor({
			element,
			content: withParagraph(document),
			editable: !disabled,
			extensions: [
				...editorExtensions({
					placeholder,
					onMetaEnter: () => (popup ? true : (onmetaenter?.() ?? false)),
				}),
				completing("slash", "/", {
					onOpen: openSlash,
					onQuery: openSlash,
					onKey: steer,
					onClose: () => closing("slash"),
				}, { command: taken }),
				completing("mention", "@", {
					onOpen: (session) => void openSearch("mention", session),
					onQuery: (session) => void openSearch("mention", session),
					onKey: steer,
					onClose: () => closing("mention"),
				}, { command: taken }),
				completing("issue", "#", {
					onOpen: (session) => void openSearch("issue", session),
					onQuery: (session) => void openSearch("issue", session),
					onKey: steer,
					onClose: () => closing("issue"),
				}, { command: taken }),
			],
			editorProps: {
				attributes: {
					class: "outline-none",
					role: "textbox",
					"aria-multiline": "true",
					"aria-label": label,
					...(id ? { id } : {}),
				},
				handlePaste: (view, event) => {
					const grabbed = take(view, fileList(event.clipboardData?.files), view.state.selection.from);

					if (grabbed) event.preventDefault();

					return grabbed;
				},
				handleDrop: (view, event) => {
					const dragged = event as DragEvent;
					const landed = view.posAtCoords({ left: dragged.clientX, top: dragged.clientY });
					const grabbed = take(
						view,
						fileList(dragged.dataTransfer?.files),
						landed?.pos ?? view.state.selection.from
					);

					if (grabbed) event.preventDefault();

					return grabbed;
				},
			},
			onUpdate: ({ editor: within }) => {
				const next = asDocument(within.getJSON());

				if (sameDocument(next, held)) return;

				held = next;
				document = next;
			},
			onTransaction: () => queueMicrotask(() => (revision += 1)),
		});
	}

	function mount(element: HTMLElement) {
		const created = build(element);

		held = document;
		editor = created;

		if (autofocus) created.commands.focus("end");

		return {
			destroy() {
				created.destroy();
				editor = null;
			},
		};
	}

	$effect(() => {
		const next = document;

		if (!editor || sameDocument(next, held)) return;

		held = next;
		editor.commands.setContent(withParagraph(next), { emitUpdate: false });
	});

	$effect(() => {
		editor?.setEditable(!disabled, false);
	});

	$effect(() => {
		const editable = editor?.view.dom;

		if (!editable) return;

		const described = rest["aria-describedby"];
		const invalid = rest["aria-invalid"];
		const active = popup && popup.rows.length > 0 ? optionId(popup.index) : undefined;

		for (const [name, value] of [
			["aria-describedby", described],
			["aria-invalid", invalid === undefined ? undefined : String(invalid)],
			["aria-activedescendant", active],
			["aria-controls", popup ? listId : undefined],
			["aria-owns", popup ? listId : undefined],
			["aria-expanded", popup ? "true" : undefined],
			["aria-autocomplete", popup ? "list" : undefined],
		] as const) {
			if (value === undefined || value === "") {
				editable.removeAttribute(name);

				continue;
			}

			editable.setAttribute(name, value);
		}
	});

</script>

<div class="flex min-w-0 flex-col {className}">
	<div class="relative min-w-0">
		<div
			use:mount
			class="{markdownProse} min-w-0 py-2 text-md [&_.ProseMirror]:{minHeight}"
			data-slot="editor"
		></div>

		{#if popup}
			<SuggestionPopup
				id={listId}
				rows={popup.rows}
				index={popup.index}
				anchor={popup.anchor}
				state={popup.state}
				label={popup.kind === "slash" ? "Blocks" : "Suggestions"}
				typingHint={popup.kind === "issue"
					? "Type a reference or a few words from the title"
					: "Type a name to find somebody, a team, an issue or a project"}
				{optionId}
				onpick={accept}
			/>
		{/if}
	</div>

	{#if toolbar}
		<Toolbar
			{editor}
			{revision}
			{disabled}
			onattach={onfiles ? () => filing?.click() : undefined}
		/>
	{/if}

	{#if onfiles}
		<input
			bind:this={filing}
			type="file"
			multiple
			class="sr-only"
			tabindex="-1"
			aria-hidden="true"
			onchange={(event) => {
				const input = event.currentTarget;
				const within = editor;

				if (within) take(within.view, fileList(input.files), within.state.selection.from);

				input.value = "";
			}}
		/>
	{/if}
</div>
