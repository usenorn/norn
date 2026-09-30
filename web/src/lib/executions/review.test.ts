import { describe, expect, it } from "vitest";
import {
	failedPreviews,
	previewFixRequest,
	previewsView,
	readingLatest,
	reviewBodyMaxLength,
	reviewPreviewLine,
	snapshotTotals,
	unchangedTitle,
	type ReviewState,
} from "./review";

function state(overrides: Partial<ReviewState>): ReviewState {
	return {
		revision: 2,
		latestRevision: 2,
		unchanged: false,
		revisions: [],
		summary: "",
		repositories: [],
		previews: [],
		heads: [],
		comments: [],
		reviews: [],
		...overrides,
	};
}

describe("review snapshots", () => {
	it("only the latest revision can be decided", () => {
		expect(readingLatest(state({ revision: 2 }))).toBe(true);
		expect(readingLatest(state({ revision: 1 }))).toBe(false);
	});

	it("says so when the Run Plan declares no previews rather than showing nothing", () => {
		expect(previewsView(state({ previews: [] }))).toEqual({ kind: "none_configured" });
	});

	it("keeps the reason a preview failed or is unsupported", () => {
		expect(
			reviewPreviewLine({ name: "API", service: "api", state: "failed", reason: "never answered /health" })
		).toContain("never answered /health");
		expect(
			reviewPreviewLine({ name: "DB", service: "postgres", state: "unsupported", reason: "compose service" })
		).toContain("compose service");
	});

	it("tells a healthy preview with no address apart from one that can be opened", () => {
		expect(reviewPreviewLine({ name: "Web", service: "web", state: "ready" })).toContain("no preview domain");
		expect(
			reviewPreviewLine({ name: "Web", service: "web", state: "ready", url: "https://web.preview.example" })
		).not.toContain("no preview domain");
	});

	it("totals every repository of the snapshot", () => {
		const totals = snapshotTotals([
			{
				repository: "api",
				branch: "norn/X-1/api",
				baseSha: "a",
				headSha: "b",
				commits: [{ sha: "b", subject: "one" }, { sha: "c", subject: "two" }],
				additions: 10,
				deletions: 2,
				filesChanged: 3,
			},
			{
				repository: "web",
				branch: "norn/X-1/web",
				baseSha: "d",
				headSha: "e",
				commits: [{ sha: "e", subject: "three" }],
				additions: 1,
				deletions: 1,
				filesChanged: 1,
			},
		]);

		expect(totals).toEqual({ repositories: 2, commits: 3, additions: 11, deletions: 3, filesChanged: 4 });
	});
});

describe("asking the agent to fix a preview", () => {
	it("names each failed preview with its reason, in the words Telegram sends", () => {
		const review = state({
			previews: [
				{ name: "API", service: "api", state: "ready" },
				{ name: "Greeting page", service: "web", state: "failed", reason: "directory not found" },
				{ name: "Docs", service: "docs", state: "failed" },
			],
		});

		expect(previewFixRequest(failedPreviews(review))).toBe(
			"These previews did not start, so nobody could try the change:\n\n" +
				"- Greeting page: directory not found\n" +
				"- Docs: no reason was given\n\n" +
				"Make them start. If the fix is in the code, change it on this branch; " +
				"if it is in how the codebase is run, say what has to change."
		);
	});

	it("stays within what a review may say", () => {
		const request = previewFixRequest(
			Array.from({ length: 32 }, (_, index) => ({
				name: `Preview ${index}`,
				service: "web",
				state: "failed" as const,
				reason: "x".repeat(2000),
			}))
		);

		expect([...request].length).toBeLessThanOrEqual(reviewBodyMaxLength);
		expect(request).not.toContain("x".repeat(301));
	});
});

describe("a revision that changed nothing", () => {
	it("names the revision it repeats", () => {
		expect(unchangedTitle(state({ revision: 3 }))).toBe("Nothing changed since revision 2");
	});
});
