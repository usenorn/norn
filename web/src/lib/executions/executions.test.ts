import { describe, expect, it } from "vitest";
import {
	canCancel,
	publicationIncomplete,
	stateLabel,
	stopCopy,
	type Execution,
	type ExecutionChangeSet,
} from "./executions";

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

describe("a run watching its pull request", () => {
	const watching = { state: "watching" } as Execution;

	it("says what it is doing", () => {
		expect(stateLabel(watching.state)).toBe("Watching the pull request");
	});

	it("can be told to stop watching without calling it stopping the run", () => {
		expect(canCancel(watching)).toBe(true);
		expect(stopCopy(watching).action).toBe("Stop watching");
		expect(stopCopy({ state: "running" } as Execution).action).toBe("Stop this run");
	});
});
