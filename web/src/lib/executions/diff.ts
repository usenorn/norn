import type { BundledLanguage } from "shiki";

export type DiffLineKind = "add" | "remove" | "context";

export type DiffSide = "old" | "new";

export const syntaxTones = [
	"foreground",
	"token-keyword",
	"token-string",
	"token-string-expression",
	"token-comment",
	"token-constant",
	"token-function",
	"token-parameter",
	"token-punctuation",
	"token-link",
] as const;

export function toneOf(index: number): string | null {
	const tone = syntaxTones[index];

	return tone ? `var(--syntax-${tone})` : null;
}

export type DiffLine = {
	kind: DiffLineKind;
	text: string;
	oldLine?: number;
	newLine?: number;
	spans?: number[];
};

export type DiffSegment = { text: string; tone: string | null };

export function segmentsOf(line: DiffLine): DiffSegment[] {
	if (!line.spans) return [{ text: line.text, tone: null }];

	const segments: DiffSegment[] = [];

	let from = 0;

	for (let index = 0; index + 1 < line.spans.length; index += 2) {
		const to = from + line.spans[index];

		segments.push({ text: line.text.slice(from, to), tone: toneOf(line.spans[index + 1]) });
		from = to;
	}

	return segments;
}

export const diffLineMax = 4000;
export const diffInlineLines = 2000;
export const diffFileInlineMax = 400;
export const diffFilesMax = 10_000;
export const diffFileMax = 5000;

const languagesByExtension: Record<string, BundledLanguage> = {
	go: "go",
	ts: "typescript",
	mts: "typescript",
	cts: "typescript",
	tsx: "tsx",
	js: "javascript",
	mjs: "javascript",
	cjs: "javascript",
	jsx: "jsx",
	svelte: "svelte",
	py: "python",
	rb: "ruby",
	rs: "rust",
	java: "java",
	kt: "kotlin",
	sql: "sql",
	json: "json",
	yaml: "yaml",
	yml: "yaml",
	toml: "toml",
	md: "markdown",
	sh: "shellscript",
	bash: "shellscript",
	css: "css",
	html: "html",
	c: "c",
	h: "c",
	cc: "cpp",
	cpp: "cpp",
	hpp: "cpp",
	swift: "swift",
	php: "php",
};

const languageOfName: Record<string, BundledLanguage> = { Dockerfile: "dockerfile" };

export const highlightedLanguages: BundledLanguage[] = [
	...new Set([...Object.values(languagesByExtension), ...Object.values(languageOfName)]),
];

export function languageOf(path: string): BundledLanguage | undefined {
	const name = path.split("/").at(-1) ?? "";

	if (name in languageOfName) return languageOfName[name];

	const dot = name.lastIndexOf(".");

	return dot > 0 ? languagesByExtension[name.slice(dot + 1).toLowerCase()] : undefined;
}

export type DiffHunk = { header: string; lines: DiffLine[] };

export type DiffFileStatus = "modified" | "added" | "deleted" | "renamed";

export type DiffFile = {
	path: string;
	oldPath: string;
	status: DiffFileStatus;
	additions: number;
	deletions: number;
	hunks: DiffHunk[];
	binary: boolean;
	deferred?: boolean;
};

export type DiffBudget = {
	inline: number;
	perFile: number;
	files: number;
	only?: string;
};

export const unbounded: DiffBudget = { inline: Infinity, perFile: Infinity, files: Infinity };

export const reviewBudget: DiffBudget = { inline: diffInlineLines, perFile: diffFileInlineMax, files: diffFilesMax };

export type DiffAnchor = { side: DiffSide; line: number };

export type SplitRow =
	| { kind: "header"; header: string }
	| { kind: "pair"; left?: DiffLine; right?: DiffLine };

export const hunkExcerptLines = 12;

const hunkHeader = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

function stripPrefix(path: string, prefix: string): string {
	return path.startsWith(prefix) ? path.slice(prefix.length) : path;
}

function pathsOfDiffHeader(line: string): { oldPath: string; path: string } {
	const named = line.slice("diff --git ".length);
	const split = named.indexOf(" b/");

	if (split === -1) return { oldPath: named, path: named };

	return { oldPath: stripPrefix(named.slice(0, split), "a/"), path: named.slice(split + 3) };
}

export class DiffReader {
	readonly files: DiffFile[] = [];
	truncated = false;
	oversized = false;

	#budget: DiffBudget;
	#spent = 0;
	#file: DiffFile | undefined;
	#hunk: DiffHunk | undefined;
	#kept = 0;
	#oldLine = 0;
	#newLine = 0;
	#done = false;

	constructor(budget: DiffBudget) {
		this.#budget = budget;
	}

	get done(): boolean {
		return this.#done;
	}

	push(line: string): void {
		if (this.#done) return;

		if (line.startsWith("diff --git ")) {
			this.#open(line);

			return;
		}

		const file = this.#file;

		if (!file) return;

		if (!this.#hunk) this.#describe(file, line);

		const opened = hunkHeader.exec(line);

		if (opened) {
			this.#hunk = { header: line, lines: [] };
			this.#oldLine = Number(opened[1]);
			this.#newLine = Number(opened[2]);

			if (!file.deferred) file.hunks.push(this.#hunk);

			return;
		}

		if (!this.#hunk || file.binary) return;

		const kind = line.startsWith("+") ? "add" : line.startsWith("-") ? "remove" : "context";

		if (kind === "context" && (line.startsWith("\\") || line === "")) return;

		const held: DiffLine =
			kind === "add"
				? { kind, text: line.slice(1), newLine: this.#newLine }
				: kind === "remove"
					? { kind, text: line.slice(1), oldLine: this.#oldLine }
					: { kind, text: line.slice(1), oldLine: this.#oldLine, newLine: this.#newLine };

		if (kind === "add") file.additions += 1;
		if (kind === "remove") file.deletions += 1;
		if (kind !== "add") this.#oldLine += 1;
		if (kind !== "remove") this.#newLine += 1;

		this.#keep(file, held);
	}

	finish(): DiffFile[] {
		this.#done = true;

		return this.files;
	}

	#open(line: string): void {
		const { oldPath, path } = pathsOfDiffHeader(line);
		const { only, files } = this.#budget;

		if (only !== undefined && this.#file) {
			this.#done = true;

			return;
		}

		this.#file = undefined;
		this.#hunk = undefined;
		this.#kept = 0;

		if (only !== undefined && path !== only) return;

		if (this.files.length >= files) {
			this.truncated = true;
			this.#done = true;

			return;
		}

		this.#file = { path, oldPath, status: "modified", additions: 0, deletions: 0, hunks: [], binary: false };
		this.files.push(this.#file);
	}

	#describe(file: DiffFile, line: string): void {
		if (line.startsWith("new file mode")) file.status = "added";
		else if (line.startsWith("deleted file mode")) file.status = "deleted";
		else if (line.startsWith("rename from ")) {
			file.status = "renamed";
			file.oldPath = line.slice("rename from ".length);
		} else if (line.startsWith("rename to ")) file.path = line.slice("rename to ".length);
		else if (line.startsWith("Binary files ") || line.startsWith("GIT binary patch")) {
			file.binary = true;
		} else if (line.startsWith("--- ")) {
			const named = line.slice(4);

			if (named !== "/dev/null") file.oldPath = stripPrefix(named, "a/");
		} else if (line.startsWith("+++ ")) {
			const named = line.slice(4);

			if (named !== "/dev/null") file.path = stripPrefix(named, "b/");
		}
	}

	#keep(file: DiffFile, line: DiffLine): void {
		if (file.deferred) return;

		const { inline, perFile, only } = this.#budget;

		if (this.#kept >= perFile || this.#spent >= inline) {
			if (only !== undefined) this.oversized = true;

			this.#spent -= this.#kept;
			this.#kept = 0;
			file.hunks = [];
			file.deferred = true;

			return;
		}

		this.#hunk?.lines.push(line);
		this.#kept += 1;
		this.#spent += 1;
	}
}

export function readDiff(lines: Iterable<string>, budget: DiffBudget): DiffReader {
	const reader = new DiffReader(budget);

	for (const line of lines) {
		reader.push(line);

		if (reader.done) break;
	}

	reader.finish();

	return reader;
}

export function parseDiff(patch: string): DiffFile[] {
	return readDiff(patch.split("\n"), unbounded).files;
}

export function anchorOf(line: DiffLine): DiffAnchor {
	if (line.kind === "remove") return { side: "old", line: line.oldLine ?? 0 };

	return { side: "new", line: line.newLine ?? 0 };
}

export function sameAnchor(left: DiffAnchor, right: DiffAnchor): boolean {
	return left.side === right.side && left.line === right.line;
}

export function lineKey(line: DiffLine): string {
	const anchor = anchorOf(line);

	return `${anchor.side}:${anchor.line}`;
}

const marks: Record<DiffLineKind, string> = { add: "+", remove: "-", context: " " };

export function hunkExcerpt(hunk: DiffHunk, upTo: DiffLine): string {
	const index = hunk.lines.indexOf(upTo);
	const kept = hunk.lines.slice(Math.max(0, index + 1 - hunkExcerptLines), index + 1);

	return kept.map((line) => marks[line.kind] + line.text).join("\n");
}

export function splitRows(hunks: DiffHunk[]): SplitRow[] {
	const rows: SplitRow[] = [];

	for (const hunk of hunks) {
		rows.push({ kind: "header", header: hunk.header });

		let removed: DiffLine[] = [];
		let added: DiffLine[] = [];

		const flush = () => {
			const length = Math.max(removed.length, added.length);

			for (let index = 0; index < length; index += 1) {
				rows.push({ kind: "pair", left: removed[index], right: added[index] });
			}

			removed = [];
			added = [];
		};

		for (const line of hunk.lines) {
			if (line.kind === "remove") {
				if (added.length > 0) flush();

				removed.push(line);
			} else if (line.kind === "add") {
				added.push(line);
			} else {
				flush();
				rows.push({ kind: "pair", left: line, right: line });
			}
		}

		flush();
	}

	return rows;
}

export function fileStatusLabel(file: DiffFile): string {
	switch (file.status) {
		case "added":
			return "Added";
		case "deleted":
			return "Deleted";
		case "renamed":
			return `Renamed from ${file.oldPath}`;
		case "modified":
			return "";
	}
}
