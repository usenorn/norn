import { describe, expect, it } from "vitest";
import { revisionDiff } from "./diff";

describe("revisionDiff", () => {
	it("numbers each side on its own so a reader can find the line in either version", () => {
		const diff = revisionDiff(
			"# Runbook\n\nRestart the worker.\n",
			"# Runbook\n\nDrain, then restart the worker.\n\nPage on-call.\n"
		);

		expect(diff.lines).toEqual([
			{ kind: "context", text: "# Runbook", oldLine: 1, newLine: 1 },
			{ kind: "context", text: "", oldLine: 2, newLine: 2 },
			{ kind: "remove", text: "Restart the worker.", oldLine: 3 },
			{ kind: "add", text: "Drain, then restart the worker.", newLine: 3 },
			{ kind: "add", text: "", newLine: 4 },
			{ kind: "add", text: "Page on-call.", newLine: 5 },
		]);
		expect(diff.additions).toBe(3);
		expect(diff.deletions).toBe(1);
	});

	it("shows a first version as nothing but additions", () => {
		const diff = revisionDiff("", "Hello\n");

		expect(diff.lines).toEqual([{ kind: "add", text: "Hello", newLine: 1 }]);
		expect(diff.deletions).toBe(0);
	});

	it("finds no change between identical versions", () => {
		const diff = revisionDiff("Same\n", "Same\n");

		expect(diff.additions + diff.deletions).toBe(0);
	});
});
