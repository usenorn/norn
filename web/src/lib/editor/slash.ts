export type SlashCommand = {
	key: string;
	label: string;
	hint: string;
	group: "Text" | "Format" | "Lists" | "Blocks" | "Insert" | "Link";
	aliases: string[];
	keys?: string;
	confirm?: string;
};

export const slashCommands: SlashCommand[] = [
	{ key: "h1", label: "Heading 1", hint: "Big section heading", group: "Text", aliases: ["heading", "title", "#"], keys: "Mod+Alt+1" },
	{ key: "h2", label: "Heading 2", hint: "Section heading", group: "Text", aliases: ["heading", "subtitle", "##"], keys: "Mod+Alt+2" },
	{ key: "h3", label: "Heading 3", hint: "Small heading", group: "Text", aliases: ["heading", "###"], keys: "Mod+Alt+3" },
	{ key: "text", label: "Text", hint: "Plain paragraph", group: "Text", aliases: ["paragraph", "body"] },
	{ key: "bold", label: "Bold", hint: "Heavy text", group: "Format", aliases: ["strong", "b", "**"], keys: "Mod+B" },
	{ key: "italic", label: "Italic", hint: "Slanted text", group: "Format", aliases: ["emphasis", "i", "_"], keys: "Mod+I" },
	{ key: "strike", label: "Strikethrough", hint: "Text with a line through it", group: "Format", aliases: ["strikethrough", "cross", "~~"], keys: "Mod+Shift+S" },
	{ key: "inlinecode", label: "Inline code", hint: "Code inside a sentence", group: "Format", aliases: ["mono", "tick", "`"], keys: "Mod+E" },
	{ key: "bullet", label: "Bulleted list", hint: "An unordered list", group: "Lists", aliases: ["list", "ul", "unordered"], keys: "Mod+Shift+8" },
	{ key: "numbered", label: "Numbered list", hint: "An ordered list", group: "Lists", aliases: ["list", "ol", "ordered"], keys: "Mod+Shift+7" },
	{ key: "todo", label: "Checklist", hint: "A list you tick off", group: "Lists", aliases: ["task", "check", "checkbox"], keys: "Mod+Shift+9" },
	{ key: "code", label: "Code block", hint: "Code with a language", group: "Blocks", aliases: ["snippet", "pre", "```"] },
	{ key: "quote", label: "Quote", hint: "A quoted passage", group: "Blocks", aliases: ["blockquote", ">"], keys: "Mod+Shift+B" },
	{ key: "divider", label: "Divider", hint: "A horizontal rule", group: "Blocks", aliases: ["rule", "hr", "---"] },
	{ key: "table", label: "Table", hint: "Three columns, a header row", group: "Blocks", aliases: ["grid"] },
	{ key: "toggle", label: "Toggle", hint: "A section that folds away", group: "Blocks", aliases: ["details", "collapse", "accordion"] },
	{ key: "image", label: "Image", hint: "Upload a picture", group: "Insert", aliases: ["picture", "photo", "upload"] },
	{ key: "file", label: "File", hint: "Attach a file", group: "Insert", aliases: ["attachment", "upload", "document"] },
	{ key: "issue", label: "Issue", hint: "Link an issue", group: "Link", aliases: ["ticket", "reference", "#"] },
	{ key: "project", label: "Project", hint: "Link a project", group: "Link", aliases: [] },
	{ key: "mention", label: "Mention", hint: "Name somebody", group: "Link", aliases: ["person", "team", "@"] },
	{
		key: "subissue",
		label: "Sub-issue",
		hint: "Raise a linked issue from the selection",
		group: "Link",
		aliases: ["child", "subtask", "break down"],
		confirm: "Raise a sub-issue",
	},
];

export function matchingCommands(query: string): SlashCommand[] {
	const asked = query.trim().toLowerCase();

	if (asked === "") return slashCommands;

	const scored = slashCommands
		.map((command) => ({ command, rank: rankOf(command, asked) }))
		.filter((held) => held.rank > 0)
		.sort((one, other) => other.rank - one.rank);

	const groups = new Map<string, SlashCommand[]>();

	for (const held of scored) {
		const kept = groups.get(held.command.group);

		if (kept) {
			kept.push(held.command);

			continue;
		}

		groups.set(held.command.group, [held.command]);
	}

	return [...groups.values()].flat();
}

function rankOf(command: SlashCommand, asked: string): number {
	if (command.key === asked) return 100;
	if (command.key.startsWith(asked)) return 80;
	if (command.label.toLowerCase().startsWith(asked)) return 70;

	for (const alias of command.aliases) {
		if (alias === asked) return 60;
		if (alias.startsWith(asked)) return 50;
	}

	if (command.label.toLowerCase().includes(asked)) return 30;
	if (command.hint.toLowerCase().includes(asked)) return 10;

	return 0;
}

export function opensMenu(before: string): boolean {
	if (before === "") return true;

	return /[\s(["']$/.test(before);
}
