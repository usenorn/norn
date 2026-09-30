import { describe, expect, it } from "vitest";
import { publicationIncomplete, type Execution, type ExecutionChangeSet } from "./executions";

function approved(): Execution {
	return { state: "approved" } as Execution;
}

function publishing(...states: (string | undefined)[]): ExecutionChangeSet {
	return {
		repositories: states.map((state) => ({ publication: state ? { state } : undefined })),
	} as ExecutionChangeSet;
}

describe("a publication that needs a person", () => {
	it("is incomplete only once nothing is still going and something failed", () => {
		expect(publicationIncomplete(approved(), publishing("published", "failed"))).toBe(true);
		expect(publicationIncomplete(approved(), publishing("pending", "failed"))).toBe(false);
		expect(publicationIncomplete(approved(), publishing(undefined, "failed"))).toBe(false);
		expect(publicationIncomplete(approved(), publishing("published", "published"))).toBe(false);
	});
});
