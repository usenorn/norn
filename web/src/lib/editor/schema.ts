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

function submitting(onMetaEnter?: () => boolean) {
	return Extension.create({
		name: "editor-submit",
		priority: 1000,
		addKeyboardShortcuts() {
			return {
				"Mod-Enter": () => onMetaEnter?.() ?? false,
			};
		},
	});
}
