import { Extension, type Extensions } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { Details, DetailsContent, DetailsSummary } from "@tiptap/extension-details";
import { Table, TableCell, TableHeader, TableRow } from "@tiptap/extension-table";
import { Placeholder } from "@tiptap/extensions";
import { Attachment } from "$lib/editor/nodes/attachment";
import { IssueRef } from "$lib/editor/nodes/issue-ref";
import { WorkspaceMention } from "$lib/editor/nodes/mention";
import { EditorUploads } from "$lib/editor/uploads";

export type SchemaOptions = {
	placeholder: string;
	onMetaEnter?: () => boolean;
};

/**
 * One schema for every editor in the product. Creation, descriptions, comments and templates
 * differ in what surrounds them, never in what a block does or which key writes it: a command
 * learnt in one is the same command everywhere.
 */
export function editorExtensions(options: SchemaOptions): Extensions {
	return [
		StarterKit.configure({
			link: { openOnClick: false, autolink: true, HTMLAttributes: { rel: "noreferrer" } },
			codeBlock: { languageClassPrefix: "language-" },
		}),
		Image.configure({ inline: false }),
		TaskList,
		TaskItem.configure({ nested: true }),
		Details.configure({ persist: true, HTMLAttributes: { class: "norn-details" } }),
		DetailsSummary,
		DetailsContent,
		Table.configure({ resizable: false }),
		TableRow,
		TableHeader,
		TableCell,
		Attachment,
		IssueRef,
		WorkspaceMention,
		EditorUploads,
		Placeholder.configure({
			placeholder: ({ node }) => (node.type.name === "detailsSummary" ? "Summary" : options.placeholder),
			showOnlyWhenEditable: true,
			showOnlyCurrent: false,
		}),
		submitting(options.onMetaEnter),
	];
}

/**
 * Cmd/Ctrl+Enter belongs to whatever owns the innermost interaction. The editor claims it only
 * when nothing inside it is open, which is why the caller answers rather than being told.
 */
function submitting(onMetaEnter?: () => boolean) {
	return Extension.create({
		name: "editor-submit",
		// Above every block extension, so the key reaches the form rather than being spent
		// leaving a code block or a list the caret happens to sit in.
		priority: 1000,
		addKeyboardShortcuts() {
			return {
				"Mod-Enter": () => onMetaEnter?.() ?? false,
			};
		},
	});
}
