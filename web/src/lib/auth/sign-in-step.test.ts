import { describe, expect, it } from "vitest";
import { methodGroups, providerLabel, soleProvider, stepFor, type SignInOptions } from "./sign-in-step";

const northwind = { workspace: "northwind", name: "Northwind", protocol: "oidc" as const, host: "northwind.okta.com" };
const labs = { workspace: "northwind-labs", name: "Northwind Labs", protocol: "saml" as const, host: "idp.labs.co" };

describe("sign-in steps", () => {
	it("opens the methods step for any address with a way in", () => {
		const options: SignInOptions = { options: [{ method: "password" }], preferred: "password" };

		expect(stepFor(options, "rae@gmail.com")).toEqual({
			kind: "methods",
			email: "rae@gmail.com",
			workspace: "",
			options: options.options,
		});
	});

	it("says so when nothing is open to the address", () => {
		expect(stepFor({ options: [] }, "rae@gmail.com")).toEqual({ kind: "closed", email: "rae@gmail.com" });
	});

	it("groups neighbouring providers so they render as one list", () => {
		const groups = methodGroups([
			{ method: "sso", workspace: northwind },
			{ method: "sso", workspace: labs },
			{ method: "password" },
		]);

		expect(groups).toEqual([{ method: "sso", workspaces: [northwind, labs] }, { method: "password" }]);
	});

	it("goes straight to a provider only when it is preferred and alone", () => {
		expect(
			soleProvider({ options: [{ method: "sso", workspace: northwind }, { method: "password" }], preferred: "sso" })
		).toEqual(northwind);

		expect(
			soleProvider({
				options: [
					{ method: "sso", workspace: northwind },
					{ method: "sso", workspace: labs },
				],
				preferred: "sso",
			})
		).toBeNull();

		expect(soleProvider({ options: [{ method: "password" }], preferred: "password" })).toBeNull();
	});

	it("names the workspace only when there is a choice to make", () => {
		expect(providerLabel(northwind, 1)).toBe("Continue with northwind.okta.com");
		expect(providerLabel(labs, 2)).toBe("Northwind Labs · idp.labs.co");
	});
});
