import type { components } from "$lib/api/dashboard.gen";
import { documentExcerpt } from "$lib/editor/document";
import type { Project } from "$lib/projects/projects";
import { onCalendarDay } from "$lib/time";
import { workspacePath } from "$lib/workspace/navigation";
import { statusLabels, type PublicRoadmapItem } from "./public-roadmap";
import type { RoadmapEntryInput } from "./roadmap-entry-schema";
import { roadmapDescriptionMaxLength, type RoadmapSettingsInput } from "./roadmap-settings-schema";

export type Roadmap = components["schemas"]["Roadmap"];
export type RoadmapEntry = components["schemas"]["RoadmapEntry"];
export type RoadmapEntryKind = components["schemas"]["RoadmapEntryKind"];
export type RoadmapEntryContent = components["schemas"]["RoadmapEntryContent"];
export type RoadmapVisibility = components["schemas"]["RoadmapVisibility"];
export type RoadmapPublication = components["schemas"]["RoadmapPublication"];
export type RoadmapReview = components["schemas"]["RoadmapReview"];
export type RoadmapEntryReview = components["schemas"]["RoadmapEntryReview"];
export type RoadmapFieldChange = components["schemas"]["RoadmapFieldChange"];
export type RoadmapReviewField = RoadmapFieldChange["field"];
export type RoadmapBlocker = components["schemas"]["RoadmapBlocker"];
export type RoadmapPreview = components["schemas"]["RoadmapPreview"];
export type RoadmapConflictCode = components["schemas"]["RoadmapConflictProblem"]["code"];
type FieldError = components["schemas"]["FieldError"];
type Issue = components["schemas"]["Issue"];

export type RoadmapSettingsView =
	| { kind: "loading" }
	| { kind: "forbidden" }
	| { kind: "unavailable" }
	| { kind: "ready"; roadmap: Roadmap; projects: Project[] };

export type RoadmapPreviewView =
	| { kind: "loading" }
	| { kind: "forbidden" }
	| { kind: "unavailable" }
	| { kind: "ready"; preview: RoadmapPreview; entries: RoadmapEntry[] };

export type RoadmapPreviewEntryView =
	| { kind: "loading" }
	| { kind: "forbidden" }
	| { kind: "not_found" }
	| { kind: "unavailable" }
	| { kind: "ready"; roadmapTitle: string; item: PublicRoadmapItem };

export type RoadmapPublicationState =
	| { kind: "unpublished"; review: RoadmapReview }
	| { kind: "published"; publication: RoadmapPublication; url: string | null; withheld: number }
	| {
			kind: "unpublished_changes";
			publication: RoadmapPublication;
			url: string | null;
			withheld: number;
			review: RoadmapReview;
	  };

export function publicationOf(roadmap: Roadmap): RoadmapPublicationState {
	const { publication, review } = roadmap;

	if (!publication) return { kind: "unpublished", review };

	const withheld = roadmap.entries.filter((entry) => entry.withheld).length;
	const published = { publication, url: roadmap.publicUrl, withheld };

	return review.hasUnpublishedChanges
		? { kind: "unpublished_changes", ...published, review }
		: { kind: "published", ...published };
}

export const publicationLabels: Record<RoadmapPublicationState["kind"], string> = {
	unpublished: "Not published",
	published: "Published",
	unpublished_changes: "Unpublished changes",
};

export type RoadmapPreset = "unstarted" | "projects" | "projects_with_issues" | "issues" | "custom";
export type ChosenPreset = Exclude<RoadmapPreset, "unstarted" | "custom">;

export const chosenPresets: ChosenPreset[] = ["projects", "projects_with_issues", "issues"];

export const presetCopy: Record<ChosenPreset, { title: string; hint: string }> = {
	projects: {
		title: "Projects only",
		hint: "Share whole projects. No issue is listed publicly.",
	},
	projects_with_issues: {
		title: "Projects with selected issues",
		hint: "Share projects and list only the issues you pick under each one.",
	},
	issues: {
		title: "Selected issues",
		hint: "Share individual issues without the projects they belong to.",
	},
};

function loose(entry: RoadmapEntry, entries: RoadmapEntry[]): boolean {
	return (
		entry.kind === "issue" &&
		!entries.some((parent) => parent.kind === "project" && parent.id === entry.parentEntryId)
	);
}

export function presetConflicts(roadmap: Roadmap, preset: ChosenPreset): RoadmapEntry[] {
	const { entries } = roadmap;

	switch (preset) {
		case "projects":
			return entries.filter((entry) => entry.kind === "issue");
		case "issues":
			return entries.filter((entry) => entry.kind === "project");
		case "projects_with_issues":
			return entries.filter((entry) => loose(entry, entries));
	}
}

export function presetOf(roadmap: Roadmap): RoadmapPreset {
	const listsIssues = roadmap.visibility.issueLists;

	if (roadmap.entries.length === 0) return listsIssues ? "projects_with_issues" : "unstarted";

	const fits = (preset: ChosenPreset) => presetConflicts(roadmap, preset).length === 0;
	const hasProject = roadmap.entries.some((entry) => entry.kind === "project");

	if (listsIssues) return hasProject && fits("projects_with_issues") ? "projects_with_issues" : "custom";
	if (fits("projects")) return "projects";
	if (fits("issues")) return "issues";

	return "custom";
}

export function presetVisibility(visibility: RoadmapVisibility, preset: ChosenPreset): RoadmapVisibility {
	return { ...visibility, issueLists: preset === "projects_with_issues" };
}

export function presetKinds(preset: RoadmapPreset): RoadmapEntryKind[] {
	switch (preset) {
		case "projects":
			return ["project"];
		case "issues":
			return ["issue"];
		default:
			return ["project", "issue"];
	}
}

export type RoadmapEntryGroup = { project: RoadmapEntry; issues: RoadmapEntry[] };
export type RoadmapEntryLayout = { projects: RoadmapEntryGroup[]; issues: RoadmapEntry[] };

function inOrder(a: RoadmapEntry, b: RoadmapEntry): number {
	if (a.position !== b.position) return a.position - b.position;

	return a.publicId < b.publicId ? -1 : a.publicId > b.publicId ? 1 : 0;
}

export function entryLayout(entries: RoadmapEntry[]): RoadmapEntryLayout {
	const ordered = [...entries].sort(inOrder);
	const projects = ordered
		.filter((entry) => entry.kind === "project")
		.map((project) => ({
			project,
			issues: ordered.filter((entry) => entry.kind === "issue" && entry.parentEntryId === project.id),
		}));

	return { projects, issues: ordered.filter((entry) => loose(entry, ordered)) };
}

export function selectedSources(roadmap: Roadmap): Set<string> {
	return new Set(
		roadmap.entries.flatMap((entry) => [entry.projectId, entry.issueId]).filter((id) => id !== undefined)
	);
}

export type EntryBadge = "not_published" | "withheld" | "out_of_reach";

export const entryBadgeLabels: Record<EntryBadge, string> = {
	not_published: "Not yet published",
	withheld: "Hidden publicly — source changed",
	out_of_reach: "Out of your reach",
};

export function entryBadges(entry: RoadmapEntry): EntryBadge[] {
	const badges: EntryBadge[] = [];

	if (!entry.published && !entry.withheld) badges.push("not_published");
	if (entry.withheld) badges.push("withheld");
	if (!entry.reachable) badges.push("out_of_reach");

	return badges;
}

export type VisibilityField = keyof RoadmapVisibility;
export type FlagEffect = "shown" | "entry_off" | "roadmap_off";

export function effectiveFlag(
	roadmap: RoadmapVisibility,
	entry: RoadmapVisibility,
	field: VisibilityField
): FlagEffect {
	if (!entry[field]) return "entry_off";
	if (!roadmap[field]) return "roadmap_off";

	return "shown";
}

export function descriptionField(kind: RoadmapEntryKind): VisibilityField {
	return kind === "project" ? "projectDescriptions" : "issueDescriptions";
}

export function entryFields(kind: RoadmapEntryKind): VisibilityField[] {
	return kind === "project"
		? ["projectDescriptions", "status", "timeline", "progress", "issueLists"]
		: ["issueDescriptions", "status", "timeline"];
}

export const visibilityLabels: Record<VisibilityField, string> = {
	issueLists: "Selected issues",
	projectDescriptions: "Project descriptions",
	issueDescriptions: "Issue descriptions",
	status: "Status",
	timeline: "Dates",
	progress: "Progress",
};

export const flagEffectLabels: Record<FlagEffect, string> = {
	shown: "Shown publicly",
	entry_off: "Hidden for this entry",
	roadmap_off: "Hidden — turned off for the whole roadmap",
};

export function progressCoverageLine(coverage: number): string | undefined {
	if (coverage === 0) return undefined;

	const issues = coverage === 1 ? "the 1 issue" : `the ${coverage} issues`;

	return `Calculated from ${issues} you selected under this project — other internal issues are not counted, and cancelled ones are left out.`;
}

export function entryContentOf(entry: RoadmapEntry): RoadmapEntryContent {
	return {
		publicTitle: entry.publicTitle,
		publicDescription: entry.publicDescription,
		status: entry.status,
		timeline: { ...entry.timeline },
		progressMode: entry.progressMode,
		progressPercent: entry.progressPercent,
		visibility: { ...entry.visibility },
		position: entry.position,
	};
}

export function newEntryContent(entries: RoadmapEntry[]): RoadmapEntryContent {
	const last = Math.max(-1, ...entries.map((entry) => entry.position));

	return {
		publicTitle: "",
		publicDescription: "",
		progressMode: "hidden",
		visibility: {
			issueLists: true,
			projectDescriptions: false,
			issueDescriptions: false,
			status: true,
			timeline: true,
			progress: true,
		},
		position: last + 1,
	};
}

export function entryInputOf(content: RoadmapEntryContent): RoadmapEntryInput {
	return {
		publicTitle: content.publicTitle,
		publicDescription: content.publicDescription ?? "",
		status: content.status ?? null,
		timeline: { startOn: content.timeline?.startOn ?? "", endOn: content.timeline?.endOn ?? "" },
		progressMode: content.progressMode,
		progressPercent: content.progressMode === "approved" ? (content.progressPercent ?? 0) : null,
		visibility: { ...content.visibility },
	};
}

export function contentOfInput(input: RoadmapEntryInput, position: number): RoadmapEntryContent {
	return {
		publicTitle: input.publicTitle,
		publicDescription: input.publicDescription,
		status: input.status ?? undefined,
		timeline: {
			startOn: input.timeline.startOn || undefined,
			endOn: input.timeline.endOn || undefined,
		},
		progressMode: input.progressMode,
		progressPercent: input.progressMode === "approved" ? (input.progressPercent ?? 0) : 0,
		visibility: { ...input.visibility },
		position,
	};
}

export function settingsInputOf(roadmap: Roadmap): RoadmapSettingsInput {
	return {
		expectedVersion: roadmap.version,
		title: roadmap.title,
		description: roadmap.description,
		...roadmap.visibility,
	};
}

export type EntryContentField = "publicTitle" | "publicDescription" | "status" | "timeline" | "progress" | "visibility";

export const entryContentLabels: Record<EntryContentField, string> = {
	publicTitle: "Public title",
	publicDescription: "Public description",
	status: "Status",
	timeline: "Dates",
	progress: "Progress",
	visibility: "What is shown",
};

export function entryDifferences(before: RoadmapEntry, after: RoadmapEntry): EntryContentField[] {
	const same = (one: object, other: object) => JSON.stringify(one) === JSON.stringify(other);
	const differences: [EntryContentField, boolean][] = [
		["publicTitle", before.publicTitle !== after.publicTitle],
		["publicDescription", before.publicDescription !== after.publicDescription],
		["status", before.status !== after.status],
		[
			"timeline",
			(before.timeline.startOn ?? "") !== (after.timeline.startOn ?? "") ||
				(before.timeline.endOn ?? "") !== (after.timeline.endOn ?? ""),
		],
		[
			"progress",
			before.progressMode !== after.progressMode || before.progressPercent !== after.progressPercent,
		],
		["visibility", !same(before.visibility, after.visibility)],
	];

	return differences.filter(([, differs]) => differs).map(([field]) => field);
}

export type ReviewSubject = { kind: "roadmap" } | { kind: "entry"; entry: RoadmapEntryReview };
export type ReviewLine = { subject: ReviewSubject; change: RoadmapFieldChange };

export type ReviewSections = {
	added: RoadmapEntryReview[];
	changed: ReviewLine[];
	exposed: ReviewLine[];
	concealed: ReviewLine[];
	removed: RoadmapEntryReview[];
};

export type ReviewSection = keyof ReviewSections;

export function reviewSections(review: RoadmapReview): ReviewSections {
	const sections: ReviewSections = { added: [], changed: [], exposed: [], concealed: [], removed: [] };

	const sort = (subject: ReviewSubject, changes: RoadmapFieldChange[]) => {
		for (const change of changes) {
			const line = { subject, change };

			if (change.kind === "changed") sections.changed.push(line);
			if (change.kind === "exposed") sections.exposed.push(line);
			if (change.kind === "concealed") sections.concealed.push(line);
		}
	};

	sort({ kind: "roadmap" }, review.roadmap);

	for (const entry of review.entries) {
		if (entry.change === "added") sections.added.push(entry);
		if (entry.change === "removed") sections.removed.push(entry);
		if (entry.change === "changed") sort({ kind: "entry", entry }, entry.fields);
	}

	return sections;
}

export const reviewSectionCopy: Record<ReviewSection, { heading: string; note: string }> = {
	added: {
		heading: "Added",
		note: "These entries appear publicly for the first time, with exactly the fields listed.",
	},
	changed: {
		heading: "Changed public text",
		note: "What readers see now, and what they will see once you publish.",
	},
	exposed: {
		heading: "Newly shown",
		note: "Fields the public roadmap does not show today.",
	},
	concealed: {
		heading: "Hidden again",
		note: "Fields the public roadmap stops showing.",
	},
	removed: {
		heading: "Removed",
		note: "These entries disappear from the public roadmap.",
	},
};

export const reviewFieldLabels: Record<RoadmapReviewField, string> = {
	title: "Title",
	description: "Description",
	status: "Status",
	startOn: "Start date",
	endOn: "End date",
	progress: "Progress",
	issues: "Issues listed",
	placement: "Listed under",
};

function isStatus(value: string): value is keyof typeof statusLabels {
	return value in statusLabels;
}

export function reviewValueLabel(field: RoadmapReviewField, value: string): string {
	switch (field) {
		case "status":
			return isStatus(value) ? statusLabels[value] : value;
		case "startOn":
		case "endOn":
			return onCalendarDay(value);
		case "progress":
			return `${value}%`;
		case "issues":
			return value === "1" ? "1 issue" : `${value} issues`;
		default:
			return value;
	}
}

export function blockerMessage(blocker: RoadmapBlocker, entries: RoadmapEntry[]): string {
	const entry = entries.find(({ id }) => id === blocker.entryId);
	const named = entry?.publicTitle || entry?.source?.label || "An entry";

	switch (blocker.code) {
		case "roadmap_title_required":
			return "Give the roadmap a public title before you publish it.";
		case "roadmap_title_too_long":
			return "Shorten the roadmap's public title to 120 characters or fewer.";
		case "entry_unreachable":
			return `${named} is left out: its project or issue was deleted, moved or is no longer within your reach.`;
		case "entry_invalid":
			return `${named} has public copy that cannot be published. Edit it to fix the problem.`;
	}
}

export type RoadmapFailure =
	| { kind: "stale"; version?: number }
	| { kind: "review_changed" }
	| { kind: "duplicate" }
	| { kind: "full" }
	| { kind: "unreachable"; entryId?: string }
	| { kind: "invalid"; errors: FieldError[] }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

const notFound = 404;
const forbidden = 403;

export function roadmapFailure(problem: unknown, status: number): RoadmapFailure {
	if (status === forbidden) return { kind: "forbidden" };
	if (status === notFound) return { kind: "gone" };

	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	const held = problem as { code?: unknown; version?: unknown; entryId?: unknown; errors?: unknown };

	switch (held.code as RoadmapConflictCode | undefined) {
		case "roadmap_stale":
			return typeof held.version === "number" ? { kind: "stale", version: held.version } : { kind: "stale" };
		case "roadmap_review_changed":
			return { kind: "review_changed" };
		case "roadmap_entry_duplicate":
			return { kind: "duplicate" };
		case "roadmap_full":
			return { kind: "full" };
		case "roadmap_unreachable":
			return typeof held.entryId === "string"
				? { kind: "unreachable", entryId: held.entryId }
				: { kind: "unreachable" };
	}

	if (Array.isArray(held.errors) && held.errors.length > 0) {
		return { kind: "invalid", errors: held.errors as FieldError[] };
	}

	return { kind: "unavailable" };
}

type PreviewAnswer<T> = { data?: T; response: Response };

export function readRoadmapPreview(
	preview: PreviewAnswer<RoadmapPreview>,
	roadmap: PreviewAnswer<Roadmap>
): RoadmapPreviewView {
	if (preview.response.status === forbidden) return { kind: "forbidden" };
	if (!preview.response.ok || !preview.data) return { kind: "unavailable" };

	return { kind: "ready", preview: preview.data, entries: roadmap.data?.entries ?? [] };
}

export function readRoadmapPreviewEntry(
	preview: PreviewAnswer<RoadmapPreview>,
	item: PreviewAnswer<PublicRoadmapItem>
): RoadmapPreviewEntryView {
	if (preview.response.status === forbidden || item.response.status === forbidden) return { kind: "forbidden" };
	if (item.response.status === notFound) return { kind: "not_found" };
	if (!preview.response.ok || !item.response.ok || !preview.data || !item.data) return { kind: "unavailable" };

	return { kind: "ready", roadmapTitle: preview.data.roadmap.title, item: item.data };
}

export function failureMessage(failure: RoadmapFailure): string {
	switch (failure.kind) {
		case "stale":
			return "Someone else changed the roadmap since you opened it. Nothing you typed was lost.";
		case "review_changed":
			return "What publishing would change is different now, because a project or issue changed. Review the new version before you publish.";
		case "duplicate":
			return "That project or issue is already on the roadmap.";
		case "full":
			return "The roadmap holds the most entries it can. Remove one before you add another.";
		case "unreachable":
			return "That project or issue was deleted, moved or is no longer within your reach.";
		case "invalid":
			return "Some public copy cannot be saved. Check the highlighted fields.";
		case "gone":
			return "That entry is no longer on the roadmap.";
		case "forbidden":
			return "You need to administer this workspace to manage its public roadmap.";
		case "unavailable":
			return "The roadmap did not change. Check your connection and try again.";
	}
}

export type RoadmapAnswer = { kind: "done"; roadmap: Roadmap } | { kind: "refused"; failure: RoadmapFailure };

export async function answerOf(
	call: Promise<{ data?: Roadmap; error?: unknown; response: Response }>
): Promise<RoadmapAnswer> {
	try {
		const { data, error, response } = await call;

		if (data) return { kind: "done", roadmap: data };

		return { kind: "refused", failure: roadmapFailure(error, response.status) };
	} catch {
		return { kind: "refused", failure: { kind: "unavailable" } };
	}
}

const fieldMessages: Partial<Record<string, string>> = {
	"title:too_long": "Shorten the public title to 120 characters or fewer.",
	"description:too_long": "Shorten the public description to 4000 characters or fewer.",
	"publicTitle:required": "Give this entry a public title.",
	"publicTitle:too_long": "Shorten the public title to 120 characters or fewer.",
	"publicDescription:too_long": "Shorten the public description to 4000 characters or fewer.",
	"status:unsupported_value": "Choose one of the listed statuses.",
	"timeline.startOn:malformed": "Use a date like 2026-09-30.",
	"timeline.endOn:malformed": "Use a date like 2026-09-30.",
	"timeline.endOn:required": "Add an end date. A start date is only shown with one.",
	"timeline.endOn:out_of_range": "End on or after the start date.",
	"progressMode:unsupported_value": "Choose how progress is shown.",
	"progressPercent:out_of_range": "Use a number from 0 to 100.",
};

export function roadmapFieldMessage(error: FieldError): string {
	return fieldMessages[`${error.field}:${error.code}`] ?? "Check this value.";
}

export type RoadmapSettingsOutcome = { kind: "saved" } | { kind: "failed"; failure: RoadmapFailure };

export type RoadmapSettingsField = Exclude<keyof RoadmapSettingsInput, "expectedVersion">;

export const roadmapSettingsFields: RoadmapSettingsField[] = [
	"title",
	"description",
	"issueLists",
	"projectDescriptions",
	"issueDescriptions",
	"status",
	"timeline",
	"progress",
];

export function settingsDraftAfter(
	baseline: RoadmapSettingsInput,
	latest: RoadmapSettingsInput,
	draft: RoadmapSettingsInput,
	touched: RoadmapSettingsField[]
): RoadmapSettingsInput | undefined {
	if (touched.some((field) => latest[field] !== baseline[field])) return undefined;

	return { ...latest, ...Object.fromEntries(touched.map((field) => [field, draft[field]])) };
}

export const issueSearchDelayMs = 250;

export type RoadmapActivity =
	| { kind: "idle" }
	| { kind: "choosing_preset"; preset: ChosenPreset }
	| { kind: "refreshing"; entryId: string }
	| { kind: "removing"; entryId: string }
	| { kind: "unpublishing" };

export type EntrySource = { id: string; kind: RoadmapEntryKind; label: string; reference?: string };

export type SourceText =
	| { kind: "loading" }
	| { kind: "ready"; label: string; reference?: string; description: string }
	| { kind: "unreadable" };

export function projectSourceText(project: Project): SourceText {
	return {
		kind: "ready",
		label: project.name,
		description: project.description.trim().slice(0, roadmapDescriptionMaxLength),
	};
}

export function issueSourceText(issue: Issue): SourceText {
	const description = issue.descriptionDoc
		? documentExcerpt(issue.descriptionDoc, roadmapDescriptionMaxLength)
		: issue.description.trim().slice(0, roadmapDescriptionMaxLength);

	return { kind: "ready", label: issue.title, reference: issue.reference, description };
}

export type EntryEditorTarget =
	| { kind: "add"; source: EntrySource; content: RoadmapEntryContent }
	| { kind: "edit"; entry: RoadmapEntry };

export type EntryEditorStanding =
	| { kind: "editing" }
	| { kind: "stale"; current: RoadmapEntry | undefined; differences: EntryContentField[]; version: number }
	| { kind: "failed"; failure: RoadmapFailure };

export type EntryEditor =
	| { kind: "closed" }
	| { kind: "open"; target: EntryEditorTarget; standing: EntryEditorStanding };

export function staleStanding(opened: RoadmapEntry | undefined, latest: Roadmap): EntryEditorStanding {
	if (!opened) return { kind: "stale", current: undefined, differences: [], version: latest.version };

	const current = latest.entries.find((entry) => entry.id === opened.id);

	return {
		kind: "stale",
		current,
		differences: current ? entryDifferences(opened, current) : [],
		version: latest.version,
	};
}

export type FrozenReview = { version: number; review: RoadmapReview };

export type ReviewDialog =
	| { kind: "closed" }
	| { kind: "loading" }
	| { kind: "ready"; frozen: FrozenReview }
	| { kind: "publishing"; frozen: FrozenReview }
	| { kind: "stale"; frozen: FrozenReview; failure: RoadmapFailure }
	| { kind: "failed"; frozen: FrozenReview; failure: RoadmapFailure }
	| { kind: "unreadable"; failure: RoadmapFailure };

export function freezeReview(roadmap: Roadmap): FrozenReview {
	return { version: roadmap.version, review: roadmap.review };
}

export function publishFailureDialog(frozen: FrozenReview, failure: RoadmapFailure): ReviewDialog {
	return failure.kind === "stale" || failure.kind === "review_changed"
		? { kind: "stale", frozen, failure }
		: { kind: "failed", frozen, failure };
}

export function roadmapSettingsPath(workspace: string): string {
	return workspacePath(workspace, "/settings/roadmap");
}

export function roadmapReviewPath(workspace: string): string {
	return `${roadmapSettingsPath(workspace)}?review=1`;
}

export function roadmapPreviewPath(workspace: string): string {
	return `${roadmapSettingsPath(workspace)}/preview`;
}

export function roadmapPreviewEntryPath(workspace: string, publicId: string): string {
	return `${roadmapPreviewPath(workspace)}/entries/${publicId}`;
}
