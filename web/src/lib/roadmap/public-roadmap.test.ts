import { describe, expect, it } from "vitest";
import {
	issueCountLabel,
	progressLabel,
	readPublicRoadmap,
	readPublicRoadmapEntry,
	timelineLabel,
	timelinePlacement,
	timelineWindow,
	type PublicRoadmap,
	type PublicRoadmapItem,
} from "./public-roadmap";

const reference = "c0ffee00-0000-4000-8000-000000000001";

const roadmap: PublicRoadmap = {
	title: "Northwind roadmap",
	publishedAt: "2026-10-01T09:00:00Z",
	projects: [{ id: "abcdefghijklmnop", kind: "project", title: "Faster checkout" }],
	issues: [],
};

function answer<T>(status: number, data?: T) {
	return { data, response: new Response(null, { status }) };
}

describe("readPublicRoadmap", () => {
	it("calls an unknown, unpublished or removed roadmap unavailable", () => {
		expect(readPublicRoadmap(answer(404), reference)).toEqual({ kind: "unavailable" });
	});

	it("keeps any other failure apart from unavailable, with a reference", () => {
		expect(readPublicRoadmap(answer(503), reference)).toEqual({ kind: "failed", reference });
		expect(readPublicRoadmap(answer(500), reference)).toEqual({ kind: "failed", reference });
	});

	it("tells an empty roadmap from one with work on it", () => {
		const empty = { ...roadmap, projects: [] };

		expect(readPublicRoadmap(answer(200, empty), reference)).toEqual({ kind: "empty", roadmap: empty });
		expect(readPublicRoadmap(answer(200, roadmap), reference)).toEqual({ kind: "ready", roadmap });
	});
});

describe("readPublicRoadmapEntry", () => {
	const item: PublicRoadmapItem = roadmap.projects[0];

	it("is unavailable when either the roadmap or the entry is gone", () => {
		expect(readPublicRoadmapEntry(answer(200, roadmap), answer(404), reference)).toEqual({
			kind: "unavailable",
		});
		expect(readPublicRoadmapEntry(answer(404), answer(404), reference)).toEqual({
			kind: "unavailable",
		});
	});

	it("reports a failure when nothing says the entry is gone", () => {
		expect(readPublicRoadmapEntry(answer(200, roadmap), answer(503), reference)).toEqual({
			kind: "failed",
			reference,
		});
	});

	it("is ready with both the roadmap and the entry", () => {
		expect(readPublicRoadmapEntry(answer(200, roadmap), answer(200, item), reference)).toEqual({
			kind: "ready",
			roadmap,
			item,
		});
	});
});

describe("timelineLabel", () => {
	it("shows only the dates that were published", () => {
		expect(timelineLabel({ startOn: "2026-10-01", endOn: "2027-01-31" })).toBe(
			"1 Oct 2026 – 31 Jan 2027"
		);
		expect(timelineLabel({ startOn: "2026-10-01" })).toBe("From 1 Oct 2026");
		expect(timelineLabel({ endOn: "2027-01-31" })).toBe("By 31 Jan 2027");
	});

	it("invents nothing when no date was published", () => {
		expect(timelineLabel({})).toBeUndefined();
		expect(timelineLabel(undefined)).toBeUndefined();
	});
});

describe("progressLabel", () => {
	it("never turns missing progress into zero", () => {
		expect(progressLabel(undefined)).toBeUndefined();
	});

	it("reads a published zero as zero", () => {
		expect(progressLabel(0)).toBe("0% complete");
		expect(progressLabel(65)).toBe("65% complete");
	});
});

describe("timeline placement", () => {
	const items: PublicRoadmapItem[] = [
		{ id: "a", kind: "project", title: "A", timeline: { startOn: "2026-01-01", endOn: "2026-01-11" } },
		{ id: "b", kind: "project", title: "B", timeline: { endOn: "2026-01-21" } },
		{ id: "c", kind: "project", title: "C" },
	];

	it("spans the earliest to the latest published date", () => {
		expect(timelineWindow(items)).toEqual({ startOn: "2026-01-01", endOn: "2026-01-21" });
		expect(timelineWindow([items[2]])).toBeUndefined();
	});

	it("places a range as a span and a single date as a marker", () => {
		const window = { startOn: "2026-01-01", endOn: "2026-01-21" };

		expect(timelinePlacement(items[0].timeline, window)).toEqual({ kind: "span", offset: 0, width: 50 });
		expect(timelinePlacement(items[1].timeline, window)).toEqual({ kind: "marker", offset: 100 });
	});

	it("gives an undated item no position at all", () => {
		expect(timelinePlacement(undefined, { startOn: "2026-01-01", endOn: "2026-01-21" })).toBeUndefined();
	});

	it("survives a window of a single day", () => {
		const window = { startOn: "2026-01-01", endOn: "2026-01-01" };

		expect(timelinePlacement({ startOn: "2026-01-01", endOn: "2026-01-01" }, window)).toEqual({
			kind: "span",
			offset: 0,
			width: 0,
		});
	});
});

describe("issueCountLabel", () => {
	it("counts only issues the roadmap chose to show", () => {
		expect(issueCountLabel({ id: "a", kind: "project", title: "A" })).toBeUndefined();
		expect(issueCountLabel({ id: "a", kind: "project", title: "A", issues: [] })).toBe("0 issues");
		expect(
			issueCountLabel({
				id: "a",
				kind: "project",
				title: "A",
				issues: [{ id: "b", kind: "issue", title: "B" }],
			})
		).toBe("1 issue");
	});
});
