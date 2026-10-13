import { describe, expect, it } from "vitest";
import { isPublicRoadmapRoute, onRoadmapHost, publicRoadmapPath } from "./host";

describe("onRoadmapHost", () => {
	it("recognises a workspace slug directly under the roadmaps domain", () => {
		expect(onRoadmapHost("acme.roadmap.norn.ink", "roadmap.norn.ink")).toBe(true);
		expect(onRoadmapHost("ACME-labs.Roadmap.Norn.Ink.", "roadmap.norn.ink")).toBe(true);
	});

	it("follows the api in refusing the bare domain, deeper hosts and invalid slugs", () => {
		expect(onRoadmapHost("roadmap.norn.ink", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost("a.b.roadmap.norn.ink", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost("a.roadmap.norn.ink", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost("-acme.roadmap.norn.ink", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost(`${"a".repeat(41)}.roadmap.norn.ink`, "roadmap.norn.ink")).toBe(false);
	});

	it("leaves the app host and look-alike hosts alone", () => {
		expect(onRoadmapHost("app.norn.so", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost("evilroadmap.norn.ink", "roadmap.norn.ink")).toBe(false);
		expect(onRoadmapHost("acme.roadmap.norn.ink.example.com", "roadmap.norn.ink")).toBe(false);
	});

	it("routes nothing when no roadmaps domain is configured", () => {
		expect(onRoadmapHost("acme.roadmap.norn.ink", undefined)).toBe(false);
		expect(onRoadmapHost("acme.roadmap.norn.ink", " ")).toBe(false);
	});
});

describe("publicRoadmapPath", () => {
	it("serves the roadmap at the root and keeps every other path beneath it", () => {
		expect(publicRoadmapPath("/")).toBe("/public-roadmap");
		expect(publicRoadmapPath("/entries/abcdefghijklmnop")).toBe(
			"/public-roadmap/entries/abcdefghijklmnop"
		);
		expect(publicRoadmapPath("/sign-in")).toBe("/public-roadmap/sign-in");
	});
});

describe("isPublicRoadmapRoute", () => {
	it("matches the public roadmap routes only", () => {
		expect(isPublicRoadmapRoute("/public-roadmap")).toBe(true);
		expect(isPublicRoadmapRoute("/public-roadmap/entries/[entryId]")).toBe(true);
		expect(isPublicRoadmapRoute("/public-roadmapper")).toBe(false);
		expect(isPublicRoadmapRoute("/[workspace]/(dashboard)/projects")).toBe(false);
		expect(isPublicRoadmapRoute(null)).toBe(false);
	});
});
