import { describe, expect, it } from "vitest";
import { grew, listed, rowsOf, taken } from "./listed";

describe("listed", () => {
	it("tells a failure apart from an empty result", () => {
		expect(listed<string>({ error: { status: 500 } }).kind).toBe("unavailable");
		expect(listed<string>({ data: { rows: [] } }).kind).toBe("empty");
	});

	it("tells an empty result apart from one the filters emptied", () => {
		expect(listed<string>({ data: { rows: [] } }).kind).toBe("empty");
		expect(listed<string>({ data: { rows: [] } }, true).kind).toBe("no_matches");
	});

	it("treats a missing answer as unavailable rather than as nothing to show", () => {
		expect(listed<string>(undefined).kind).toBe("unavailable");
		expect(listed<string>({}).kind).toBe("unavailable");
	});

	it("carries the cursor through so the caller knows more remains", () => {
		const page = listed({ data: { rows: ["a"], nextCursor: "next" } });

		expect(page.kind === "ready" && page.nextCursor).toBe("next");
	});
});

describe("taken", () => {
	it("reads an undefined list as unavailable, which is what a failed load leaves", () => {
		expect(taken(undefined).kind).toBe("unavailable");
		expect(taken<string>([]).kind).toBe("empty");
		expect(taken<string>([], true).kind).toBe("no_matches");
		expect(taken(["a"]).kind).toBe("ready");
	});
});

describe("grew", () => {
	it("keeps what was already loaded when another page arrives", () => {
		const first = listed({ data: { rows: ["a", "b"], nextCursor: "next" } });
		const more = grew(first, { rows: ["c"] });

		expect(rowsOf(more)).toEqual(["a", "b", "c"]);
		expect(more.kind === "ready" && more.nextCursor).toBeUndefined();
	});
});
