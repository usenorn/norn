import { beforeEach, describe, expect, it } from "vitest";
import { expectedVersion, forget, remember } from "./versions";

const issue = { id: "one", version: 3 };

describe("the version an edit claims to be editing", () => {
	beforeEach(() => forget());

	it("is the loaded one until something newer comes back", () => {
		expect(expectedVersion(issue)).toBe(3);
	});

	it("follows the answer to the last edit, so a second move is not called a conflict", () => {
		remember({ ...issue, version: 4 });

		expect(expectedVersion(issue)).toBe(4);
	});

	it("never goes backwards when a stale list arrives after a newer answer", () => {
		remember({ ...issue, version: 7 });

		expect(expectedVersion({ id: "one", version: 5 })).toBe(7);
	});

	it("prefers the loaded version when that is the newer of the two", () => {
		remember({ ...issue, version: 4 });

		expect(expectedVersion({ id: "one", version: 9 })).toBe(9);
	});

	it("keeps one issue's version out of another's", () => {
		remember({ ...issue, version: 8 });

		expect(expectedVersion({ id: "other", version: 2 })).toBe(2);
	});

	it("ignores an answer that is not an issue, such as the list of labels", () => {
		remember([{ id: "label", name: "bug" }]);
		remember({ id: "one" });
		remember(null);

		expect(expectedVersion(issue)).toBe(3);
	});
});
