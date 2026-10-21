import { describe, expect, it } from "vitest";
import { changeLine, type ActivityChange } from "./activity";

function labels(fromValue: string, toValue: string): ActivityChange {
	return {
		id: "6f1c2a4e-9b7d-4c1e-8a2f-3d5b7c9e1f20",
		kind: "property_changed",
		field: "labels",
		fromValue,
		toValue,
	};
}

describe("label changes", () => {
	it("names only the label that was added", () => {
		expect(changeLine(labels("Bug", "Bug, Flaky"))).toBe("Added the label Flaky");
	});

	it("names only the labels that were removed", () => {
		expect(changeLine(labels("Bug, Flaky, Infra", "Flaky"))).toBe(
			"Removed the labels Bug, Infra"
		);
	});

	it("says both when one label replaces another", () => {
		expect(changeLine(labels("Bug", "Feature"))).toBe(
			"Added the label Feature and removed the label Bug"
		);
	});

	it("reads the first label on an issue that had none", () => {
		expect(changeLine(labels("", "Bug"))).toBe("Added the label Bug");
	});
});
