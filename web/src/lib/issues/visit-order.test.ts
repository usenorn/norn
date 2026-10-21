import { describe, expect, it } from "vitest";
import { neighboursIn, orderFor, rememberOrder } from "./visit-order";

describe("orderFor", () => {
	it("answers only for the list the order was remembered from", () => {
		rememberOrder("/acme/issues?tab=active", ["BIL-3", "BIL-1", "BIL-2"]);

		expect(orderFor("/acme/issues?tab=active")).toEqual(["BIL-3", "BIL-1", "BIL-2"]);
		expect(orderFor("/acme/issues?tab=backlog")).toBeNull();
	});
});

describe("neighboursIn", () => {
	it("counts the position within the list the issue was opened from", () => {
		expect(neighboursIn(["BIL-3", "BIL-1", "BIL-2"], "BIL-1")).toEqual({
			position: 1,
			total: 3,
			previous: "BIL-3",
			next: "BIL-2",
		});
	});

	it("has no neighbour past either end", () => {
		expect(neighboursIn(["BIL-3", "BIL-1"], "BIL-3")?.previous).toBeNull();
		expect(neighboursIn(["BIL-3", "BIL-1"], "BIL-1")?.next).toBeNull();
	});

	it("gives nothing when the issue is not in the remembered list", () => {
		expect(neighboursIn(["BIL-3"], "BIL-9")).toBeNull();
		expect(neighboursIn(null, "BIL-9")).toBeNull();
	});
});
