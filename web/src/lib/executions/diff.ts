import type { BundledLanguage } from "shiki";

export type DiffLineKind = "add" | "remove" | "context";

export type DiffSide = "old" | "new";

export type DiffToken = { text: string; tone: string };

export type DiffLine = {
	kind: DiffLineKind;
	text: string;
	oldLine?: number;
	newLine?: number;
	tokens?: DiffToken[];
};

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
};

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

export function parseDiff(patch: string): DiffFile[] {
	const files: DiffFile[] = [];

	let file: DiffFile | undefined;
	let hunk: DiffHunk | undefined;
	let oldLine = 0;
	let newLine = 0;

	for (const line of patch.split("\n")) {
		if (line.startsWith("diff --git ")) {
			const { oldPath, path } = pathsOfDiffHeader(line);

			file = { path, oldPath, status: "modified", additions: 0, deletions: 0, hunks: [], binary: false };
			hunk = undefined;
			files.push(file);

			continue;
		}

		if (!file) continue;

		if (!hunk) {
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

		const opened = hunkHeader.exec(line);

		if (opened) {
			hunk = { header: line, lines: [] };
			oldLine = Number(opened[1]);
			newLine = Number(opened[2]);
			file.hunks.push(hunk);

			continue;
		}

		if (!hunk || file.binary) continue;

		if (line.startsWith("+")) {
			file.additions += 1;
			hunk.lines.push({ kind: "add", text: line.slice(1), newLine });
			newLine += 1;

			continue;
		}

		if (line.startsWith("-")) {
			file.deletions += 1;
			hunk.lines.push({ kind: "remove", text: line.slice(1), oldLine });
			oldLine += 1;

			continue;
		}

		if (line.startsWith("\\") || line === "") continue;

		hunk.lines.push({ kind: "context", text: line.slice(1), oldLine, newLine });
		oldLine += 1;
		newLine += 1;
	}

	return files;
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
