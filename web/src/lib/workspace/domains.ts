import type { Client } from "openapi-fetch";
import type { components, paths } from "#lib/api/dashboard.gen.js";

export type WorkspaceDomain = components["schemas"]["WorkspaceDomain"];

export type DomainList = { kind: "available"; domains: WorkspaceDomain[] } | { kind: "unavailable" };

export type DomainFailure = "taken" | "personal" | "malformed" | "lookup_failed" | "unavailable";

export type DomainStatus =
	| { kind: "verified"; at: string }
	| { kind: "waiting" }
	| { kind: "not_found"; checkedAt: string };

type Problem = components["schemas"]["Problem"];

export const domainFailureCopy: Record<DomainFailure, string> = {
	taken: "This workspace has already added that domain.",
	personal: "Personal mail domains like gmail.com can't be claimed.",
	malformed: "Enter a domain like northwind.co.",
	lookup_failed: "The domain's DNS couldn't be read. Try again in a minute.",
	unavailable: "Something went wrong. Nothing changed.",
};

export async function readDomains(api: Client<paths>, workspaceId: string): Promise<DomainList> {
	const { data } = await api.GET("/workspaces/{workspaceId}/sso/domains", {
		params: { path: { workspaceId } },
	});

	return data ? { kind: "available", domains: data } : { kind: "unavailable" };
}

export function domainStatus(domain: WorkspaceDomain): DomainStatus {
	if (domain.verifiedAt) return { kind: "verified", at: domain.verifiedAt };
	if (domain.checkedAt) return { kind: "not_found", checkedAt: domain.checkedAt };

	return { kind: "waiting" };
}

export function domainFailure(problem: Problem): DomainFailure {
	if (problem.status === 409) return "taken";
	if (problem.status === 503) return "lookup_failed";

	const code = problem.errors?.find((field) => field.field === "domain")?.code;

	if (code === "personal_domain") return "personal";
	if (code) return "malformed";

	return "unavailable";
}
