<script lang="ts">
	import type { Editor } from "@tiptap/core";
	import AtSign from "@lucide/svelte/icons/at-sign";
	import Bold from "@lucide/svelte/icons/bold";
	import Code from "@lucide/svelte/icons/code";
	import Hash from "@lucide/svelte/icons/hash";
	import Heading2 from "@lucide/svelte/icons/heading-2";
	import Italic from "@lucide/svelte/icons/italic";
	import List from "@lucide/svelte/icons/list";
	import ListChecks from "@lucide/svelte/icons/list-checks";
	import ListOrdered from "@lucide/svelte/icons/list-ordered";
	import Paperclip from "@lucide/svelte/icons/paperclip";
	import Quote from "@lucide/svelte/icons/quote";
	import Strikethrough from "@lucide/svelte/icons/strikethrough";
	import { Button } from "$lib/components/ui/button/index.js";
	import { shortcutLabel } from "$lib/editor/keys";

	let {
		editor,
		revision,
		disabled = false,
		onattach,
	}: {
		editor: Editor | null;
		revision: number;
		disabled?: boolean;
		onattach?: () => void;
	} = $props();

	function active(name: string, attributes?: Record<string, unknown>): boolean {
		revision;

		return editor?.isActive(name, attributes) ?? false;
	}

	const controls = $derived([
		{
			key: "bold",
			label: `Bold (${shortcutLabel("b")})`,
			icon: Bold,
			on: active("bold"),
			run: () => editor?.chain().focus().toggleBold().run(),
		},
		{
			key: "italic",
			label: `Italic (${shortcutLabel("i")})`,
			icon: Italic,
			on: active("italic"),
			run: () => editor?.chain().focus().toggleItalic().run(),
		},
		{
			key: "strike",
			label: "Strikethrough",
			icon: Strikethrough,
			on: active("strike"),
			run: () => editor?.chain().focus().toggleStrike().run(),
		},
		{
			key: "code",
			label: `Inline code (${shortcutLabel("e")})`,
			icon: Code,
			on: active("code"),
			run: () => editor?.chain().focus().toggleCode().run(),
		},
		{
			key: "heading",
			label: "Heading",
			icon: Heading2,
			on: active("heading", { level: 2 }),
			run: () => editor?.chain().focus().toggleHeading({ level: 2 }).run(),
		},
		{
			key: "bullets",
			label: "Bulleted list",
			icon: List,
			on: active("bulletList"),
			run: () => editor?.chain().focus().toggleBulletList().run(),
		},
		{
			key: "numbers",
			label: "Numbered list",
			icon: ListOrdered,
			on: active("orderedList"),
			run: () => editor?.chain().focus().toggleOrderedList().run(),
		},
		{
			key: "tasks",
			label: "Checklist",
			icon: ListChecks,
			on: active("taskList"),
			run: () => editor?.chain().focus().toggleTaskList().run(),
		},
		{
			key: "quote",
			label: "Quote",
			icon: Quote,
			on: active("blockquote"),
			run: () => editor?.chain().focus().toggleBlockquote().run(),
		},
	]);

	function trigger(char: string) {
		editor?.chain().focus().insertContent(char).run();
	}

	let holder = $state.raw<HTMLDivElement | null>(null);
	let at = $state(0);

	function steer(event: KeyboardEvent) {
		const buttons = [...(holder?.querySelectorAll("button") ?? [])];

		if (buttons.length === 0) return;

		const step =
			event.key === "ArrowRight" || event.key === "ArrowDown"
				? 1
				: event.key === "ArrowLeft" || event.key === "ArrowUp"
					? -1
					: 0;

		if (step === 0 && event.key !== "Home" && event.key !== "End") return;

		event.preventDefault();

		const next =
			event.key === "Home"
				? 0
				: event.key === "End"
					? buttons.length - 1
					: (at + step + buttons.length) % buttons.length;

		at = next;
		buttons[next].focus();
	}
</script>

<div
	bind:this={holder}
	class="-order-1 flex flex-wrap items-center gap-0.5 border-b border-line-subtle pb-1"
	role="toolbar"
	aria-label="Formatting"
	tabindex={-1}
	onkeydown={steer}
>
	{#each controls as control, index (control.key)}
		<Button
			type="button"
			variant="ghost"
			size="icon-sm"
			tabindex={index === at ? 0 : -1}
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
	{#if onattach}
		<Button
			type="button"
			variant="ghost"
			size="icon-sm"
			tabindex={at === controls.length ? 0 : -1}
			{disabled}
			aria-label="Attach a file"
			onclick={onattach}
		>
			<Paperclip aria-hidden="true" />
		</Button>
	{/if}
	<Button
		type="button"
		variant="ghost"
		size="icon-sm"
		tabindex={at === controls.length + (onattach ? 1 : 0) ? 0 : -1}
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
		tabindex={at === controls.length + (onattach ? 2 : 1) ? 0 : -1}
		{disabled}
		aria-label="Link an issue"
		onclick={() => trigger("#")}
	>
		<Hash aria-hidden="true" />
	</Button>
</div>
