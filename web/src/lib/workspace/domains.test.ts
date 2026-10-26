import { describe, expect, it } from "vitest";
import { domainFailure, domainStatus, emailRouting } from "./domains";

const base = { type: "about:blank", title: "", status: 422 };

describe("domain claims", () => {
	it("reads its status from when it was verified or last checked", () => {
		const claim = { id: "1", domain: "northwind.co", record: "norn-verification=abc" };

		expect(domainStatus(claim)).toEqual({ kind: "waiting" });
		expect(domainStatus({ ...claim, checkedAt: "2026-10-26T09:00:00Z" })).toEqual({
			kind: "not_found",
			checkedAt: "2026-10-26T09:00:00Z",
		});
		expect(
			domainStatus({ ...claim, checkedAt: "2026-10-26T09:00:00Z", verifiedAt: "2026-10-26T09:00:00Z" })
		).toEqual({ kind: "verified", at: "2026-10-26T09:00:00Z" });
	});

	it("routes email sign-in to a tested provider only once a domain is verified", () => {
		const pending = { id: "1", domain: "northwind.co", record: "norn-verification=abc" };
		const verified = { ...pending, id: "2", verifiedAt: "2026-10-26T09:00:00Z" };

		expect(emailRouting({ kind: "available", domains: [] }, true)).toEqual({ kind: "unrouted" });
		expect(emailRouting({ kind: "available", domains: [pending] }, true)).toEqual({ kind: "unrouted" });
		expect(emailRouting({ kind: "available", domains: [pending, verified] }, true)).toEqual({
			kind: "routed",
		});
		expect(emailRouting({ kind: "available", domains: [] }, false)).toEqual({ kind: "unknown" });
		expect(emailRouting({ kind: "unavailable" }, true)).toEqual({ kind: "unknown" });
	});

	it("names why a claim was refused", () => {
		expect(domainFailure({ ...base, status: 409 })).toBe("taken");
		expect(domainFailure({ ...base, status: 503 })).toBe("lookup_failed");
		expect(domainFailure({ ...base, errors: [{ field: "domain", code: "personal_domain" }] })).toBe("personal");
		expect(domainFailure({ ...base, errors: [{ field: "domain", code: "malformed" }] })).toBe("malformed");
		expect(domainFailure({ ...base, status: 500 })).toBe("unavailable");
	});
});
