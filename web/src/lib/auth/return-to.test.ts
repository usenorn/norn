import { describe, expect, it } from "vitest";
import { actionPath } from "./return-to";

describe("action paths", () => {
	it("keep the page's query so the load after the action sees it", () => {
		expect(actionPath(new URL("http://norn.test/sign-in?add=1&return=%2Fx"), "identify")).toBe(
			"?/identify&add=1&return=%2Fx"
		);
	});

	it("drop an action the page was posted to", () => {
		expect(actionPath(new URL("http://norn.test/sign-in?/identify&add=1"), "password")).toBe(
			"?/password&add=1"
		);
		expect(actionPath(new URL("http://norn.test/sign-in"), "identify")).toBe("?/identify");
	});
});
