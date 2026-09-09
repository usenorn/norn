<script lang="ts">
	import { untrack } from "svelte";
	import { Editor, Extension } from "@tiptap/core";
	import StarterKit from "@tiptap/starter-kit";
	import { Markdown } from "@tiptap/markdown";
	import Image from "@tiptap/extension-image";
	import Suggestion from "@tiptap/suggestion";
	import { PluginKey } from "@tiptap/pm/state";
	import type { EditorView } from "@tiptap/pm/view";
	import AtSign from "@lucide/svelte/icons/at-sign";
	import Bold from "@lucide/svelte/icons/bold";
	import Bot from "@lucide/svelte/icons/bot";
	import Code from "@lucide/svelte/icons/code";
	import Hash from "@lucide/svelte/icons/hash";
	import Heading2 from "@lucide/svelte/icons/heading-2";
	import Italic from "@lucide/svelte/icons/italic";
	import List from "@lucide/svelte/icons/list";
	import ListOrdered from "@lucide/svelte/icons/list-ordered";
	import Quote from "@lucide/svelte/icons/quote";
	import Strikethrough from "@lucide/svelte/icons/strikethrough";
	import { Button } from "$lib/components/ui/button/index.js";
	import { markdownProse } from "$lib/issues/markdown";
	import {
		DescriptionUploads,
		dropUpload,
		placeUpload,
		previewOf,
		uploadPosition,
	} from "$lib/issues/description-uploads";
	import {
		issueItems,
		mentionItems,
		type MentionPerson,
		type SuggestionItem,
	} from "$lib/issues/description-suggestions";
	import type { Team } from "$lib/team/teams";

	let {
		value = $bindable(""),
		members = [],
		teams = [],
		workspaceId,
		workspace,
		placeholder = "Add description…",
		disabled = false,
		autofocus = false,
		id,
		onfiles,
		onmetaenter,
		class: className = "",
		...rest
	}: {
		value?: string;
		members?: MentionPerson[];
		teams?: Team[];
		workspaceId: string;
		workspace: string;
		placeholder?: string;
		disabled?: boolean;
		autofocus?: boolean;
		id?: string;
		onfiles?: (files: File[]) => string[] | void;
		onmetaenter?: () => void;
		class?: string;
		"aria-describedby"?: string;
		"aria-invalid"?: boolean | "true" | "false";
	} = $props();

	type Popup = {
		items: SuggestionItem[];
		index: number;
		left: number;
		top: number;
		take: (item: SuggestionItem) => void;
	};

	let host = $state<HTMLDivElement | null>(null);
	let editor = $state.raw<Editor | null>(null);
	let popup = $state.raw<Popup | null>(null);
	let revision = $state(0);
	let emitted = "";
	let sought = 0;

	const listId = $props.id();
	const optionId = (at: number) => `${listId}-option-${at}`;

	function completing(char: string, key: string) {
		return Extension.create({
			name: `describe-${key}`,
			addProseMirrorPlugins() {
				return [
					Suggestion<SuggestionItem>({
						editor: this.editor,
						char,
						pluginKey: new PluginKey(`describe-${key}`),
						allowSpaces: char === "#",
						items: () => [],
						command: ({ editor: within, range, props }) => {
							within
								.chain()
								.focus()
								.insertContentAt(range, [
									{
										type: "text",
										text: props.text,
										marks: props.href
											? [{ type: "link", attrs: { href: props.href } }]
											: undefined,
									},
									{ type: "text", text: " " },
								])
								.run();
						},
						render: () => ({
							onStart: (props) => void show(key, props),
							onUpdate: (props) => void show(key, props),
							onKeyDown: ({ event }) => steer(event),
							onExit: () => (popup = null),
						}),
					}),
				];
			},
		});
	}

	async function offer(key: string, query: string): Promise<SuggestionItem[]> {
		if (key === "mention") return mentionItems(members, teams, query);

		return issueItems(workspaceId, workspace, query);
	}

	async function show(
		key: string,
		props: {
			query: string;
			command: (item: SuggestionItem) => void;
			clientRect?: (() => DOMRect | null) | null;
		}
	) {
		const rect = props.clientRect?.();

		if (!rect) {
			popup = null;

			return;
		}

		const asked = (sought += 1);
		const items = await offer(key, props.query);

		if (asked !== sought) return;

		popup = items.length > 0 ? { items, index: 0, left: rect.left, top: rect.bottom, take: props.command } : null;
	}

	function steer(event: KeyboardEvent): boolean {
		if (!popup) return false;

		if (event.key === "ArrowDown" || event.key === "ArrowUp") {
			const step = event.key === "ArrowDown" ? 1 : -1;
			const next = (popup.index + step + popup.items.length) % popup.items.length;

			popup = { ...popup, index: next };

			return true;
		}

		if (event.key === "Enter" || event.key === "Tab") {
			popup.take(popup.items[popup.index]);

			return true;
		}

		if (event.key === "Escape") {
			popup = null;

			return true;
		}

		return false;
	}

	function files(list: FileList | null | undefined): File[] {
		return Array.from(list ?? []);
	}

	function take(view: EditorView, dropped: File[], pos: number): boolean {
		if (dropped.length === 0 || !onfiles) return false;

		const ids = onfiles(dropped) ?? [];

		ids.forEach((taskId, at) => {
			const file = dropped[at];

			placeUpload(view, { id: taskId, name: file.name, preview: previewOf(file) }, pos);
		});

		return true;
	}

	export function settle(taskId: string, markdown: string): boolean {
		const within = editor;

		if (!within) return false;

		const pos = uploadPosition(within.state, taskId);

		if (pos === null) return false;

		dropUpload(within.view, taskId);
		within.commands.insertContentAt(pos, markdown, { contentType: "markdown" });

		return true;
	}

	export function abandon(taskId: string) {
		if (editor) dropUpload(editor.view, taskId);
	}

	function written(within: Editor): string {
		return within.getMarkdown().replace(/[ \t]+$/gm, "").trim();
	}

	function build(element: HTMLElement): Editor {
		return new Editor({
			element,
			content: value,
			contentType: "markdown",
			editable: !disabled,
			extensions: [
				StarterKit.configure({
					link: { openOnClick: false, autolink: true, HTMLAttributes: { rel: "noreferrer" } },
				}),
				Markdown,
				Image.configure({ inline: false }),
				completing("@", "mention"),
				completing("#", "issue"),
				DescriptionUploads,
			],
			editorProps: {
				attributes: {
					class: "min-h-16 outline-none",
					role: "textbox",
					"aria-multiline": "true",
					"aria-label": "Description",
					...(id ? { id } : {}),
				},
				handlePaste: (view, event) => {
					const taken = take(view, files(event.clipboardData?.files), view.state.selection.from);

					if (taken) event.preventDefault();

					return taken;
				},
				handleDrop: (view, event) => {
					const dragged = event as DragEvent;
					const landed = view.posAtCoords({ left: dragged.clientX, top: dragged.clientY });
					const taken = take(
						view,
						files(dragged.dataTransfer?.files),
						landed?.pos ?? view.state.selection.from
					);

					if (taken) event.preventDefault();

					return taken;
				},
				handleKeyDown: (_view, event) => {
					if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && onmetaenter) {
						event.preventDefault();
						onmetaenter();

						return true;
					}

					return false;
				},
			},
			onUpdate: ({ editor: within }) => {
				const next = written(within);

				if (next === emitted) return;

				emitted = next;
				value = next;
			},
			onTransaction: () => (revision += 1),
		});
	}

	$effect(() => {
		const element = host;

		if (!element) return;

		const created = untrack(() => build(element));

		emitted = untrack(() => value);
		editor = created;

		untrack(() => {
			if (autofocus) created.commands.focus("end");
		});

		return () => {
			created.destroy();
			editor = null;
		};
	});

	$effect(() => {
		const next = value;

		if (!editor || next === emitted) return;

		emitted = next;
		editor.commands.setContent(next, { contentType: "markdown", emitUpdate: false });
	});

	$effect(() => {
		editor?.setEditable(!disabled, false);
	});

	$effect(() => {
		const editable = editor?.view.dom;

		if (!editable) return;

		const described = rest["aria-describedby"];
		const invalid = rest["aria-invalid"];
		const active = popup ? optionId(popup.index) : undefined;

		for (const [name, held] of [
			["aria-describedby", described],
			["aria-invalid", invalid === undefined ? undefined : String(invalid)],
			["aria-activedescendant", active],
			["aria-controls", popup ? listId : undefined],
			["aria-expanded", popup ? "true" : undefined],
		] as const) {
			if (held === undefined || held === "") {
				editable.removeAttribute(name);

				continue;
			}

			editable.setAttribute(name, held);
		}
	});

	const empty = $derived(value.trim() === "");

	function active(name: string, attributes?: Record<string, unknown>): boolean {
		revision;

		return editor?.isActive(name, attributes) ?? false;
	}

	const controls = $derived([
		{ key: "bold", label: "Bold", icon: Bold, on: active("bold"), run: () => editor?.chain().focus().toggleBold().run() },
		{ key: "italic", label: "Italic", icon: Italic, on: active("italic"), run: () => editor?.chain().focus().toggleItalic().run() },
		{ key: "strike", label: "Strikethrough", icon: Strikethrough, on: active("strike"), run: () => editor?.chain().focus().toggleStrike().run() },
		{ key: "code", label: "Code", icon: Code, on: active("code"), run: () => editor?.chain().focus().toggleCode().run() },
		{ key: "heading", label: "Heading", icon: Heading2, on: active("heading", { level: 2 }), run: () => editor?.chain().focus().toggleHeading({ level: 2 }).run() },
		{ key: "bullets", label: "Bulleted list", icon: List, on: active("bulletList"), run: () => editor?.chain().focus().toggleBulletList().run() },
		{ key: "numbers", label: "Numbered list", icon: ListOrdered, on: active("orderedList"), run: () => editor?.chain().focus().toggleOrderedList().run() },
		{ key: "quote", label: "Quote", icon: Quote, on: active("blockquote"), run: () => editor?.chain().focus().toggleBlockquote().run() },
	]);

	function trigger(char: string) {
		editor?.chain().focus().insertContent(char).run();
	}
</script>

<div class="flex min-w-0 flex-col {className}">
	<div
		class="flex flex-wrap items-center gap-0.5 border-b border-line-subtle pb-1"
		role="toolbar"
		aria-label="Description formatting"
	>
		{#each controls as control (control.key)}
			<Button
				type="button"
				variant="ghost"
				size="icon-sm"
				{disabled}
				aria-label={control.label}
				aria-pressed={control.on}
				class={control.on ? "bg-accent text-ink-900" : ""}
				onclick={control.run}
			>
				<control.icon aria-hidden="true" />
			</Button>
		{/each}
		<span class="flex-1"></span>
		<Button
			type="button"
			variant="ghost"
			size="icon-sm"
			{disabled}
			aria-label="Mention somebody"
			onclick={() => trigger("@")}
		>
			<AtSign aria-hidden="true" />
		</Button>
		<Button
			type="button"
			variant="ghost"
			size="icon-sm"
			{disabled}
			aria-label="Link an issue"
			onclick={() => trigger("#")}
		>
			<Hash aria-hidden="true" />
		</Button>
	</div>

	<div class="relative min-w-0">
		<div
			bind:this={host}
			class="{markdownProse} min-w-0 py-2 text-md [&_.ProseMirror]:min-h-16"
			data-slot="description-editor"
		></div>

		{#if empty}
			<p class="pointer-events-none absolute top-2 left-0 text-md text-muted-foreground">
				{placeholder}
			</p>
		{/if}

		{#if popup}
			<ul
				id={listId}
				role="listbox"
				aria-label="Suggestions"
				class="fixed z-50 max-h-56 w-72 overflow-y-auto rounded-md border border-line-strong bg-popover p-1 shadow-md"
				style="left: {popup.left}px; top: {popup.top}px"
			>
				{#each popup.items as item, at (item.key)}
					<li>
						<button
							type="button"
							id={optionId(at)}
							role="option"
							aria-selected={at === popup.index}
							class="flex w-full items-center gap-1.5 rounded-sm px-2 py-1 text-left text-sm {at ===
							popup.index
								? 'bg-accent text-ink-900'
								: 'text-ink-900'}"
							onmousedown={(event) => {
								event.preventDefault();
								popup?.take(item);
							}}
						>
							{#if item.agent}
								<Bot class="size-3.5 text-muted-foreground" aria-label="An agent" />
							{/if}
							<span class="min-w-0 flex-1 truncate">{item.label}</span>
							{#if item.hint}
								<span class="font-mono text-2xs text-muted-foreground">{item.hint}</span>
							{/if}
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</div>
