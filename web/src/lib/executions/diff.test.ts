import { describe, expect, it } from "vitest";
import { anchorOf, hunkExcerpt, parseDiff, readDiff, splitRows } from "./diff";

const patch = [
	"diff --git a/internal/run.go b/internal/run.go",
	"index 1111111..2222222 100644",
	"--- a/internal/run.go",
	"+++ b/internal/run.go",
	"@@ -10,4 +10,5 @@ func run() error {",
	" \tctx := context.Background()",
	"-\treturn nil",
	"+\tif err := start(ctx); err != nil {",
	"+\t\treturn err",
	"+\t}",
	" }",
	"@@ -40,2 +41,2 @@ func stop() {",
	"-\tclose(done)",
	"+\tcancel()",
	" }",
	"\\ No newline at end of file",
	"diff --git a/old.txt b/new.txt",
	"similarity index 90%",
	"rename from old.txt",
	"rename to new.txt",
	"diff --git a/logo.png b/logo.png",
	"new file mode 100644",
	"Binary files /dev/null and b/logo.png differ",
	"diff --git a/gone.go b/gone.go",
	"deleted file mode 100644",
	"--- a/gone.go",
	"+++ /dev/null",
	"@@ -1,1 +0,0 @@",
	"-package gone",
	"",
].join("\n");

describe("parseDiff", () => {
	const files = parseDiff(patch);

	it("numbers every line on the side it belongs to", () => {
		const [first] = files[0].hunks;

		expect(first.lines.map((line) => [line.kind, line.oldLine, line.newLine])).toEqual([
			["context", 10, 10],
			["remove", 11, undefined],
			["add", undefined, 11],
			["add", undefined, 12],
			["add", undefined, 13],
			["context", 12, 14],
		]);
		expect(files[0].hunks[1].lines[1]).toMatchObject({ kind: "add", newLine: 41 });
	});

	it("counts what each file added and removed", () => {
		expect(files[0]).toMatchObject({ additions: 4, deletions: 2, status: "modified" });
	});

	it("keeps two hunks that share a header apart", () => {
		expect(files[0].hunks).toHaveLength(2);
	});

	it("names renamed, added, binary and deleted files for what they are", () => {
		expect(files[1]).toMatchObject({ status: "renamed", oldPath: "old.txt", path: "new.txt" });
		expect(files[2]).toMatchObject({ status: "added", binary: true, path: "logo.png" });
		expect(files[3]).toMatchObject({ status: "deleted", path: "gone.go", deletions: 1 });
	});
});

describe("anchors and excerpts", () => {
	const [file] = parseDiff(patch);
	const [hunk] = file.hunks;

	it("anchors a removed line to the old side and everything else to the new side", () => {
		expect(anchorOf(hunk.lines[1])).toEqual({ side: "old", line: 11 });
		expect(anchorOf(hunk.lines[2])).toEqual({ side: "new", line: 11 });
		expect(anchorOf(hunk.lines[0])).toEqual({ side: "new", line: 10 });
	});

	it("quotes the hunk up to the commented line and no further", () => {
		expect(hunkExcerpt(hunk, hunk.lines[2])).toBe(
			" \tctx := context.Background()\n-\treturn nil\n+\tif err := start(ctx); err != nil {"
		);
	});
});

describe("splitRows", () => {
	it("pairs removals with the additions that replaced them", () => {
		const [file] = parseDiff(patch);
		const rows = splitRows([file.hunks[0]]);

		expect(rows[0]).toEqual({ kind: "header", header: file.hunks[0].header });
		expect(rows.slice(1).map((row) => row.kind === "pair" && [row.left?.kind, row.right?.kind])).toEqual([
			["context", "context"],
			["remove", "add"],
			[undefined, "add"],
			[undefined, "add"],
			["context", "context"],
		]);
	});
});

describe("reading a diff within a budget", () => {
	const fileWith = (index: number, lines: number) =>
		[`diff --git a/f${index}.ts b/f${index}.ts`, `@@ -0,0 +1,${lines} @@`]
			.concat(Array.from({ length: lines }, (_, line) => `+line ${line}`))
			.join("\n");

	const patch = [fileWith(0, 3), fileWith(1, 50), fileWith(2, 3), fileWith(3, 3)].join("\n");

	it("keeps small files whole and defers the ones past the budget, counting every one", () => {
		const read = readDiff(patch.split("\n"), { inline: 8, perFile: 10, files: 100 });

		expect(read.files.map((file) => [file.path, file.deferred ?? false, file.additions])).toEqual([
			["f0.ts", false, 3],
			["f1.ts", true, 50],
			["f2.ts", false, 3],
			["f3.ts", true, 3],
		]);
		expect(read.files.filter((file) => file.deferred).every((file) => file.hunks.length === 0)).toBe(true);
	});

	it("lists no more files than it may, and says so", () => {
		const read = readDiff(patch.split("\n"), { inline: 100, perFile: 100, files: 2 });

		expect(read.files).toHaveLength(2);
		expect(read.truncated).toBe(true);
	});

	it("reads only the file asked for and stops after it", () => {
		const read = readDiff(patch.split("\n"), { inline: Infinity, perFile: Infinity, files: Infinity, only: "f2.ts" });

		expect(read.files.map((file) => file.path)).toEqual(["f2.ts"]);
		expect(read.files[0].hunks[0].lines).toHaveLength(3);
		expect(read.done).toBe(true);
	});
});
