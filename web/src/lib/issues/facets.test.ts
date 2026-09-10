import { describe, expect, it } from "vitest";
import { dueBucketOf, facetFilters, readFacets } from "./facets";
import type { IssueFilter } from "./filter";

const today = "2026-09-10";

function admits(filter: IssueFilter | undefined, due: string): boolean {
	if (!filter) return true;

	if ("all" in filter && filter.all) return filter.all.every((part) => admits(part, due));

	const [value] = filter.values ?? [];

	switch (filter.op) {
		case "before":
			return due < value;
		case "after":
			return due > value;
		case "on":
			return due === value;
		default:
			return true;
	}
}

function windowFor(window: string, due: string): boolean {
	const [filter] = facetFilters({ due: window }, today);

	return admits(filter, due);
}

describe("due buckets and the due filter", () => {
	it("puts each day in exactly one bucket", () => {
		expect(dueBucketOf("2026-09-09", today)).toBe("overdue");
		expect(dueBucketOf(today, today)).toBe("today");
		expect(dueBucketOf("2026-09-11", today)).toBe("week");
		expect(dueBucketOf("2026-09-17", today)).toBe("week");
		expect(dueBucketOf("2026-09-18", today)).toBe("later");
		expect(dueBucketOf(undefined, today)).toBe("none");
	});

	it("filters to the same days the bucket of that name holds", () => {
		for (const due of ["2026-09-09", today, "2026-09-11", "2026-09-17", "2026-09-18"]) {
			const bucket = dueBucketOf(due, today);

			expect({ due, admitted: windowFor(bucket, due) }).toEqual({ due, admitted: true });

			for (const other of ["overdue", "today", "week", "later"]) {
				if (other === bucket) continue;

				expect({ due, other, admitted: windowFor(other, due) }).toEqual({
					due,
					other,
					admitted: false,
				});
			}
		}
	});
});

describe("readFacets", () => {
	it("reads only what the URL asserts", () => {
		const held = readFacets(new URLSearchParams("priority=high&due=today&nonsense=1"));

		expect(held).toEqual({ priority: "high", due: "today" });
	});
});
