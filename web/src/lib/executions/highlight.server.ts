import { createCssVariablesTheme, createHighlighter, type Highlighter } from "shiki";
import { languageOf, type DiffFile, type DiffHunk, type DiffLine, type DiffToken } from "./diff";

const theme = createCssVariablesTheme({ name: "norn-syntax", variablePrefix: "--syntax-", fontStyle: false });

const languages = [
	"go",
	"typescript",
	"tsx",
	"javascript",
	"jsx",
	"svelte",
	"python",
	"ruby",
	"rust",
	"java",
	"kotlin",
	"sql",
	"json",
	"yaml",
	"toml",
	"markdown",
	"shellscript",
	"css",
	"html",
	"c",
	"cpp",
	"swift",
	"php",
	"dockerfile",
] as const;

type Language = (typeof languages)[number];

const loaded = new Set<string>(languages);

function known(language: string | undefined): language is Language {
	return language !== undefined && loaded.has(language);
}

let loading: Promise<Highlighter> | null = null;

function highlighter(): Promise<Highlighter> {
	loading ??= createHighlighter({ themes: [theme], langs: [...languages] });

	return loading;
}

export async function highlighted(files: DiffFile[]): Promise<DiffFile[]> {
	if (!files.some((file) => known(languageOf(file.path)))) return files;

	const shiki = await highlighter();

	return files.map((file) => {
		const language = languageOf(file.path);

		if (!known(language) || file.binary) return file;

		return { ...file, hunks: file.hunks.map((hunk) => highlightHunk(shiki, language, hunk)) };
	});
}

function highlightHunk(shiki: Highlighter, language: Language, hunk: DiffHunk): DiffHunk {
	const lines = hunk.lines.map((line) => ({ ...line }));

	for (const side of ["old", "new"] as const) {
		const kept = lines.filter((line) => line.kind === "context" || line.kind === (side === "old" ? "remove" : "add"));

		tokenize(shiki, language, kept);
	}

	return { ...hunk, lines };
}

function tokenize(shiki: Highlighter, language: Language, lines: DiffLine[]) {
	if (lines.length === 0) return;

	const tokenized = shiki.codeToTokensBase(lines.map((line) => line.text).join("\n"), {
		lang: language,
		theme,
	});

	lines.forEach((line, index) => {
		if (line.tokens) return;

		const tokens: DiffToken[] = (tokenized[index] ?? []).map((token) => ({
			text: token.content,
			tone: token.color ?? "",
		}));

		if (tokens.map((token) => token.text).join("") === line.text) line.tokens = tokens;
	});
}
