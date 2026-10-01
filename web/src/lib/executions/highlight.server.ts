import { createCssVariablesTheme, createHighlighter, type BundledLanguage, type Highlighter } from "shiki";
import { highlightedLanguages, languageOf, syntaxTones, type DiffFile, type DiffHunk, type DiffLine } from "./diff";

const theme = createCssVariablesTheme({ name: "norn-syntax", variablePrefix: "--syntax-", fontStyle: false });

export const highlightLinesMax = 2000;
export const highlightLineWidthMax = 1000;

const unhighlighted = [/\.lock$/, /(^|\/)package-lock\.json$/, /(^|\/)pnpm-lock\.yaml$/, /\.min\.[a-z]+$/, /\.map$/];

let loading: Promise<Highlighter> | null = null;

function highlighter(): Promise<Highlighter> {
	loading ??= createHighlighter({ themes: [theme], langs: highlightedLanguages });

	return loading;
}

function linesOf(file: DiffFile): number {
	return file.hunks.reduce((total, hunk) => total + hunk.lines.length, 0);
}

export function highlightable(file: DiffFile): boolean {
	if (file.binary || file.deferred || !languageOf(file.path)) return false;
	if (unhighlighted.some((pattern) => pattern.test(file.path))) return false;

	const lines = linesOf(file);

	if (lines === 0 || lines > highlightLinesMax) return false;

	return file.hunks.every((hunk) => hunk.lines.every((line) => line.text.length <= highlightLineWidthMax));
}

export async function highlighted(files: DiffFile[], budget = highlightLinesMax): Promise<DiffFile[]> {
	const chosen = new Set<DiffFile>();

	let left = budget;

	for (const file of files) {
		const lines = linesOf(file);

		if (lines <= left && highlightable(file)) {
			chosen.add(file);
			left -= lines;
		}
	}

	if (chosen.size === 0) return files;

	const shiki = await highlighter();

	return files.map((file) => {
		const language = languageOf(file.path);

		if (!chosen.has(file) || !language) return file;

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

const toneIndex = new Map<string, number>(syntaxTones.map((tone, index) => [`var(--syntax-${tone})`, index]));

function tokenize(shiki: Highlighter, language: BundledLanguage, lines: DiffLine[]) {
	if (lines.length === 0) return;

	const tokenized = shiki.codeToTokensBase(lines.map((line) => line.text).join("\n"), {
		lang: language,
		theme,
	});

	lines.forEach((line, index) => {
		if (line.spans) return;

		const spans: number[] = [];

		let covered = 0;

		for (const token of tokenized[index] ?? []) {
			spans.push(token.content.length, toneIndex.get(token.color ?? "") ?? -1);
			covered += token.content.length;
		}

		if (covered === line.text.length) line.spans = spans;
	});
}
