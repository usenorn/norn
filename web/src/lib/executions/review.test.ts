import { describe, expect, it } from "vitest";
import { previewsView, readingLatest, reviewPreviewLine, snapshotTotals, type ReviewState } from "./review";

function state(overrides: Partial<ReviewState>): ReviewState {
	return {
		revision: 2,
		latestRevision: 2,
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
