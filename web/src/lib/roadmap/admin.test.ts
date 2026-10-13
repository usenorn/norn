import { describe, expect, it } from "vitest";
import {
	contentOfInput,
	effectiveFlag,
	entryBadges,
	entryContentOf,
	entryDifferences,
	entryInputOf,
	entryLayout,
	intendedPreset,
	issueSourceText,
	newEntryContent,
	presetConflicts,
	presetKinds,
	presetOf,
	presetVisibility,
	progressCoverageLine,
	publicationOf,
	publishFailureDialog,
	readRoadmapPreview,
	readRoadmapPreviewEntry,
	reviewSections,
	reviewToPublish,
	reviewValueLabel,
	roadmapFailure,
	roadmapFieldMessage,
	selectedSources,
	settingsDraftAfter,
	settingsInputOf,
	staleStanding,
	type Roadmap,
	type RoadmapEntry,
	type RoadmapPreview,
	type RoadmapReview,
	type RoadmapVisibility,
} from "./admin";
import { roadmapEntrySchema } from "./roadmap-entry-schema";
import { roadmapDescriptionMaxLength } from "./roadmap-settings-schema";

const hidden: RoadmapVisibility = {
	issueLists: false,
	projectDescriptions: false,
	issueDescriptions: false,
	status: false,
	timeline: false,
	progress: false,
};

const shown: RoadmapVisibility = {
	issueLists: true,
	projectDescriptions: true,
	issueDescriptions: true,
	status: true,
	timeline: true,
	progress: true,
};

const quiet: RoadmapReview = { hasUnpublishedChanges: false, digest: "d1", roadmap: [], entries: [], blockers: [] };

function entry(overrides: Partial<RoadmapEntry> & Pick<RoadmapEntry, "id" | "kind">): RoadmapEntry {
	return {
		publicId: `pub-${overrides.id}`,
		publicTitle: `Entry ${overrides.id}`,
		publicDescription: "",
		status: "planned",
		timeline: {},
		progressMode: "hidden",
		progressPercent: 0,
		visibility: { ...shown },
		position: 0,
		reachable: true,
		published: false,
		withheld: false,
		progressCoverage: 0,
		...overrides,
	};
}

function roadmap(overrides: Partial<Roadmap> = {}): Roadmap {
	return {
		id: "roadmap",
		title: "Northwind roadmap",
		description: "",
		visibility: { ...hidden },
		version: 3,
		entries: [],
		review: quiet,
		publicUrl: null,
		...overrides,
	};
}

const checkout = entry({ id: "p1", kind: "project", projectId: "project-1" });
const nested = entry({ id: "i1", kind: "issue", issueId: "issue-1", parentEntryId: "p1" });
const loose = entry({ id: "i2", kind: "issue", issueId: "issue-2" });

const publication = {
	title: "Northwind roadmap",
	description: "",
	visibility: { ...hidden },
	version: 2,
	publishedAt: "2026-10-01T09:00:00Z",
};

describe("reading where the publication stands", () => {
	it("is unpublished until something was published, whatever the review says", () => {
		const review = { ...quiet, hasUnpublishedChanges: true };

		expect(publicationOf(roadmap({ review }))).toEqual({ kind: "unpublished", url: null, review });
	});

	it("keeps the public address before the first publication", () => {
		const state = publicationOf(roadmap({ review: quiet, publicUrl: "https://roadmap.example.com" }));

		expect(state).toEqual({ kind: "unpublished", url: "https://roadmap.example.com", review: quiet });
	});

	it("takes unpublished changes from the server's review, not from a local diff", () => {
		const review = { ...quiet, hasUnpublishedChanges: true };
		const state = publicationOf(roadmap({ publication, review, publicUrl: "https://roadmap.example.com" }));

		expect(state).toEqual({
			kind: "unpublished_changes",
			publication,
			url: "https://roadmap.example.com",
			withheld: 0,
			review,
		});
	});

	it("counts the entries whose published copy is withheld", () => {
		const entries = [entry({ id: "a", kind: "project", withheld: true }), checkout];

		expect(publicationOf(roadmap({ publication, entries }))).toEqual({
			kind: "published",
			publication,
			url: null,
			withheld: 1,
		});
	});
});

describe("reading the preset from what is selected", () => {
	it("calls an empty roadmap unstarted unless it already lists issues under projects", () => {
		expect(presetOf(roadmap())).toBe("unstarted");
		expect(presetOf(roadmap({ visibility: { ...hidden, issueLists: true } }))).toBe("projects_with_issues");
	});

	it("tells the three presets apart by entry kinds and the roadmap's issue lists", () => {
		expect(presetOf(roadmap({ entries: [checkout] }))).toBe("projects");
		expect(presetOf(roadmap({ entries: [loose] }))).toBe("issues");
		expect(presetOf(roadmap({ entries: [checkout, nested], visibility: { ...hidden, issueLists: true } }))).toBe(
			"projects_with_issues"
		);
	});

	it("falls back to custom when the selection fits no preset", () => {
		expect(presetOf(roadmap({ entries: [checkout, loose] }))).toBe("custom");
		expect(presetOf(roadmap({ entries: [checkout, loose], visibility: { ...hidden, issueLists: true } }))).toBe(
			"custom"
		);
		expect(presetOf(roadmap({ entries: [loose], visibility: { ...hidden, issueLists: true } }))).toBe("custom");
	});

	it("names the entries a preset conflicts with instead of removing them", () => {
		const current = roadmap({ entries: [checkout, nested, loose] });

		expect(presetConflicts(current, "projects")).toEqual([nested, loose]);
		expect(presetConflicts(current, "issues")).toEqual([checkout]);
		expect(presetConflicts(current, "projects_with_issues")).toEqual([loose]);
	});

	it("keeps a chosen preset only while the roadmap's issue lists agree with it", () => {
		const projectsOnly = roadmap({ entries: [checkout] });
		const listing = roadmap({ entries: [checkout], visibility: { ...hidden, issueLists: true } });

		expect(intendedPreset(projectsOnly, "issues")).toBe("issues");
		expect(intendedPreset(projectsOnly, "projects_with_issues")).toBe("projects");
		expect(intendedPreset(listing, "projects")).toBe("projects_with_issues");
		expect(intendedPreset(roadmap({ entries: [checkout, loose] }), undefined)).toBeUndefined();
	});

	it("only ever changes the roadmap's issue lists when a preset is applied", () => {
		expect(presetVisibility(shown, "projects")).toEqual({ ...shown, issueLists: false });
		expect(presetVisibility(hidden, "projects_with_issues")).toEqual({ ...hidden, issueLists: true });
	});

	it("limits what can be added to the kinds the preset shares", () => {
		expect(presetKinds("projects")).toEqual(["project"]);
		expect(presetKinds("issues")).toEqual(["issue"]);
		expect(presetKinds("custom")).toEqual(["project", "issue"]);
	});
});

describe("laying out the selected entries", () => {
	it("nests issues under their project and keeps the others loose, in position order", () => {
		const later = entry({ id: "p2", kind: "project", position: 5 });
		const orphan = entry({ id: "i3", kind: "issue", parentEntryId: "gone", position: 1 });

		expect(entryLayout([loose, later, nested, orphan, checkout])).toEqual({
			projects: [
				{ project: checkout, issues: [nested] },
				{ project: later, issues: [] },
			],
			issues: [loose, orphan],
		});
	});

	it("knows which projects and issues are already selected", () => {
		expect(selectedSources(roadmap({ entries: [checkout, loose] }))).toEqual(new Set(["project-1", "issue-2"]));
	});

	it("badges entries the public cannot see yet, withheld copy and sources out of reach", () => {
		expect(entryBadges(checkout)).toEqual(["not_published"]);
		expect(entryBadges({ ...checkout, published: true })).toEqual([]);
		expect(entryBadges({ ...checkout, published: true, withheld: true, reachable: false })).toEqual([
			"withheld",
			"out_of_reach",
		]);
	});
});

describe("explaining a visibility switch", () => {
	it("blames the entry first, then the roadmap", () => {
		expect(effectiveFlag(hidden, hidden, "timeline")).toBe("entry_off");
		expect(effectiveFlag(hidden, shown, "timeline")).toBe("roadmap_off");
		expect(effectiveFlag(shown, shown, "timeline")).toBe("shown");
	});

	it("says nothing about calculated progress when no selected issue counts", () => {
		expect(progressCoverageLine(0)).toBeUndefined();
		expect(progressCoverageLine(1)).toContain("the 1 issue you selected");
		expect(progressCoverageLine(4)).toContain("the 4 issues you selected");
	});
});

describe("writing entry content", () => {
	it("always carries status, timeline and position, because an update replaces the entry wholesale", () => {
		const placed = entry({
			id: "p9",
			kind: "project",
			status: "in_progress",
			timeline: { startOn: "2026-11-01", endOn: "2026-12-01" },
			position: 7,
		});

		expect(entryContentOf(placed)).toMatchObject({
			status: "in_progress",
			timeline: { startOn: "2026-11-01", endOn: "2026-12-01" },
			position: 7,
		});
	});

	it("keeps descriptions private by default and places a new entry last", () => {
		const content = newEntryContent([checkout, { ...loose, position: 4 }]);

		expect(content.position).toBe(5);
		expect(content.progressMode).toBe("hidden");
		expect(content.visibility).toMatchObject({
			projectDescriptions: false,
			issueDescriptions: false,
			issueLists: true,
		});
		expect(content.status).toBeUndefined();
	});

	it("round-trips through the form without inventing a status or dates", () => {
		const content = newEntryContent([]);
		const input = entryInputOf({ ...content, publicTitle: "Faster checkout" });

		expect(input.status).toBeNull();
		expect(contentOfInput(input, 0)).toEqual({
			...content,
			publicTitle: "Faster checkout",
			publicDescription: "",
			status: undefined,
			timeline: { startOn: undefined, endOn: undefined },
			progressPercent: 0,
		});
	});

	it("names what someone else changed on the entry", () => {
		const theirs = { ...checkout, publicTitle: "Renamed", visibility: { ...shown, timeline: false } };

		expect(entryDifferences(checkout, theirs)).toEqual(["publicTitle", "visibility"]);
	});

	it("reports an entry removed meanwhile as stale with nothing to compare", () => {
		expect(staleStanding(checkout, roadmap({ version: 9 }))).toEqual({
			kind: "stale",
			current: undefined,
			differences: [],
			version: 9,
		});
	});
});

describe("validating an entry's public copy", () => {
	const valid = entryInputOf({ ...newEntryContent([]), publicTitle: "Faster checkout" });

	it("needs an end date with a start date, not before it", () => {
		const startOnly = roadmapEntrySchema.safeParse({ ...valid, timeline: { startOn: "2026-11-01", endOn: "" } });
		const backwards = roadmapEntrySchema.safeParse({
			...valid,
			timeline: { startOn: "2026-11-02", endOn: "2026-11-01" },
		});

		expect(startOnly.success).toBe(false);
		expect(startOnly.error?.issues[0].path).toEqual(["timeline", "endOn"]);
		expect(backwards.success).toBe(false);
	});

	it("needs a percentage before approved progress can be shown", () => {
		expect(roadmapEntrySchema.safeParse({ ...valid, progressMode: "approved" }).success).toBe(false);
		expect(roadmapEntrySchema.safeParse({ ...valid, progressMode: "approved", progressPercent: 40 }).success).toBe(
			true
		);
	});

	it("refuses an empty public title", () => {
		expect(roadmapEntrySchema.safeParse({ ...valid, publicTitle: "   " }).success).toBe(false);
	});
});

describe("sorting the review into what a reader will notice", () => {
	it("separates added and removed entries from field changes, roadmap copy included", () => {
		const added = {
			entryId: "p1",
			publicId: "pub-p1",
			kind: "project" as const,
			title: "Faster checkout",
			change: "added" as const,
			fields: [{ field: "title" as const, kind: "exposed" as const, after: "Faster checkout" }],
		};
		const changed = {
			publicId: "pub-i1",
			kind: "issue" as const,
			title: "Saved carts",
			change: "changed" as const,
			fields: [
				{ field: "endOn" as const, kind: "exposed" as const, after: "2026-12-01" },
				{ field: "description" as const, kind: "concealed" as const, before: "Internal-ish" },
			],
		};
		const removed = { ...added, entryId: "p2", publicId: "pub-p2", change: "removed" as const };
		const retitled = { field: "title" as const, kind: "changed" as const, before: "Old", after: "New" };

		const sections = reviewSections({
			...quiet,
			hasUnpublishedChanges: true,
			roadmap: [retitled],
			entries: [added, changed, removed],
		});

		expect(sections.added).toEqual([added]);
		expect(sections.removed).toEqual([removed]);
		expect(sections.changed).toEqual([{ subject: { kind: "roadmap" }, change: retitled }]);
		expect(sections.exposed).toEqual([{ subject: { kind: "entry", entry: changed }, change: changed.fields[0] }]);
		expect(sections.concealed).toEqual([{ subject: { kind: "entry", entry: changed }, change: changed.fields[1] }]);
	});

	it("reads the server's values the way the public page shows them", () => {
		expect(reviewValueLabel("status", "in_progress")).toBe("In progress");
		expect(reviewValueLabel("progress", "40")).toBe("40%");
		expect(reviewValueLabel("issues", "1")).toBe("1 issue");
		expect(reviewValueLabel("placement", "Faster checkout")).toBe("Faster checkout");
	});
});

describe("reading a refused roadmap change", () => {
	it("keeps the version a stale write was refused at", () => {
		expect(roadmapFailure({ status: 409, code: "roadmap_stale", version: 8 }, 409)).toEqual({
			kind: "stale",
			version: 8,
		});
	});

	it("tells a changed review apart from a stale version", () => {
		expect(roadmapFailure({ status: 409, code: "roadmap_review_changed" }, 409)).toEqual({
			kind: "review_changed",
		});
	});

	it("keeps the field errors of a refused entry", () => {
		const errors = [{ field: "timeline.endOn", code: "out_of_range" }];

		expect(roadmapFailure({ status: 422, errors }, 422)).toEqual({ kind: "invalid", errors });
	});

	it("reads forbidden and missing from the status, and anything else as unavailable", () => {
		expect(roadmapFailure({ status: 403, code: "forbidden" }, 403)).toEqual({ kind: "forbidden" });
		expect(roadmapFailure(undefined, 404)).toEqual({ kind: "gone" });
		expect(roadmapFailure(undefined, 500)).toEqual({ kind: "unavailable" });
	});

	it("keeps the review dialog open in a stale state when the review moved under it", () => {
		const frozen = { version: 3, review: quiet };

		expect(publishFailureDialog(frozen, { kind: "review_changed" }).kind).toBe("stale");
		expect(publishFailureDialog(frozen, { kind: "stale", version: 4 }).kind).toBe("stale");
		expect(publishFailureDialog(frozen, { kind: "unavailable" }).kind).toBe("failed");
	});

	it("lets a publish that never reached the server be sent again, and re-reads after any other refusal", () => {
		const frozen = { version: 3, review: quiet };

		expect(reviewToPublish({ kind: "ready", frozen })).toBe(frozen);
		expect(reviewToPublish(publishFailureDialog(frozen, { kind: "unavailable" }))).toBe(frozen);
		expect(reviewToPublish(publishFailureDialog(frozen, { kind: "unreachable", entryId: "e1" }))).toBeUndefined();
		expect(reviewToPublish(publishFailureDialog(frozen, { kind: "stale", version: 4 }))).toBeUndefined();
	});
});

describe("keeping a settings draft while the roadmap moves underneath it", () => {
	const baseline = settingsInputOf(roadmap({ version: 3 }));

	it("adopts the server's version and untouched fields, and keeps what was typed", () => {
		const latest = settingsInputOf(roadmap({ version: 4, visibility: { ...hidden, issueLists: true } }));
		const draft = { ...baseline, title: "Where Northwind is heading" };

		expect(settingsDraftAfter(baseline, latest, draft, ["title"])).toEqual({
			...latest,
			title: "Where Northwind is heading",
		});
	});

	it("refuses to merge when someone else changed a field that was typed, so saving meets the conflict", () => {
		const latest = settingsInputOf(roadmap({ version: 4, title: "Their title" }));
		const draft = { ...baseline, title: "My title" };

		expect(settingsDraftAfter(baseline, latest, draft, ["title"])).toBeUndefined();
	});
});

describe("explaining a refused field", () => {
	it("speaks to the field and the reason, and falls back for anything unknown", () => {
		expect(roadmapFieldMessage({ field: "timeline.endOn", code: "out_of_range" })).toBe(
			"End on or after the start date."
		);
		expect(roadmapFieldMessage({ field: "kind", code: "unsupported_value" })).toBe("Check this value.");
	});
});

describe("reading the draft preview", () => {
	const draft: RoadmapPreview = {
		version: 3,
		digest: "d1",
		roadmap: { title: "Northwind roadmap", publishedAt: "0001-01-01T00:00:00Z", projects: [], issues: [] },
		blockers: [{ code: "entry_unreachable", entryId: "p1" }],
	};
	const item = { id: "pub-p1", kind: "project" as const, title: "Faster checkout" };

	function answer<T>(data: T | undefined, status: number) {
		return { data, response: new Response(null, { status }) };
	}

	it("keeps only the blocked admin entries, so a blocker can name what was left out", () => {
		expect(readRoadmapPreview(answer(draft, 200), answer(roadmap({ entries: [checkout, loose] }), 200))).toEqual({
			kind: "ready",
			preview: draft,
			entries: [checkout],
		});
	});

	it("still renders the preview when the entries could not be read", () => {
		expect(readRoadmapPreview(answer(draft, 200), answer(undefined, 500))).toEqual({
			kind: "ready",
			preview: draft,
			entries: [],
		});
	});

	it("tells a member without the roadmap grant apart from a failure", () => {
		expect(readRoadmapPreview(answer(undefined, 403), answer(undefined, 403)).kind).toBe("forbidden");
		expect(readRoadmapPreview(answer(undefined, 500), answer(roadmap(), 200)).kind).toBe("unavailable");
	});

	it("titles an entry with the draft roadmap's public title", () => {
		expect(readRoadmapPreviewEntry(answer(draft, 200), answer(item, 200))).toEqual({
			kind: "ready",
			roadmapTitle: "Northwind roadmap",
			item,
		});
	});

	it("reads an entry the draft does not show as not found, not as a failure", () => {
		expect(readRoadmapPreviewEntry(answer(draft, 200), answer(undefined, 404)).kind).toBe("not_found");
		expect(readRoadmapPreviewEntry(answer(undefined, 403), answer(undefined, 403)).kind).toBe("forbidden");
		expect(readRoadmapPreviewEntry(answer(undefined, 500), answer(item, 200)).kind).toBe("unavailable");
	});
});

describe("copying an issue description into the public copy", () => {
	it("fits the public description limit with its ellipsis", () => {
		const issue = {
			title: "Offline edits",
			reference: "ENG-7",
			description: "",
			descriptionDoc: {
				type: "doc",
				content: [{ type: "paragraph", content: [{ type: "text", text: `${"a".repeat(4000)} b` }] }],
			},
		} as Parameters<typeof issueSourceText>[0];

		const text = issueSourceText(issue);

		expect(text.kind === "ready" && text.description.length).toBe(roadmapDescriptionMaxLength);
	});
});
