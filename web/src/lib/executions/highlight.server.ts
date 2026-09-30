import { createCssVariablesTheme, createHighlighter, type BundledLanguage, type Highlighter } from "shiki";
import {
	highlightedLanguages,
	languageOf,
	type DiffFile,
	type DiffHunk,
	type DiffLine,
	type DiffToken,
} from "./diff";

const theme = createCssVariablesTheme({ name: "norn-syntax", variablePrefix: "--syntax-", fontStyle: false });

let loading: Promise<Highlighter> | null = null;

function highlighter(): Promise<Highlighter> {
	loading ??= createHighlighter({ themes: [theme], langs: highlightedLanguages });

	return loading;
}

export async function highlighted(files: DiffFile[]): Promise<DiffFile[]> {
	if (!files.some((file) => languageOf(file.path))) return files;

	const shiki = await highlighter();

	return files.map((file) => {
		const language = languageOf(file.path);

		if (!language || file.binary) return file;

		return { ...file, hunks: file.hunks.map((hunk) => highlightHunk(shiki, language, hunk)) };
	});
}

function highlightHunk(shiki: Highlighter, language: BundledLanguage, hunk: DiffHunk): DiffHunk {
	const lines = hunk.lines.map((line) => ({ ...line }));

	for (const side of ["old", "new"] as const) {
		const kept = lines.filter((line) => line.kind === "context" || line.kind === (side === "old" ? "remove" : "add"));

		tokenize(shiki, language, kept);
	}

	return { ...hunk, lines };
}

function tokenize(shiki: Highlighter, language: BundledLanguage, lines: DiffLine[]) {
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
