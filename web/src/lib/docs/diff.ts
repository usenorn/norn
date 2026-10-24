import { diffLines } from "diff";
import type { DiffLine } from "$lib/executions/diff";

export type RevisionDiff = { lines: DiffLine[]; additions: number; deletions: number };

function linesOf(value: string): string[] {
	const lines = value.split("\n");

	if (lines.at(-1) === "") lines.pop();

	return lines;
}

export function revisionDiff(before: string, after: string): RevisionDiff {
	const lines: DiffLine[] = [];
	let oldLine = 1;
	let newLine = 1;
	let additions = 0;
	let deletions = 0;

	for (const change of diffLines(before, after)) {
		for (const text of linesOf(change.value)) {
			if (change.added) {
				lines.push({ kind: "add", text, newLine: newLine++ });
				additions++;
			} else if (change.removed) {
				lines.push({ kind: "remove", text, oldLine: oldLine++ });
				deletions++;
			} else {
				lines.push({ kind: "context", text, oldLine: oldLine++, newLine: newLine++ });
			}
		}
	}

	return { lines, additions, deletions };
}
