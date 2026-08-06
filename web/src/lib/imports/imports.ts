import type { components, operations } from "$lib/api/dashboard.gen";
import type { LabelColor } from "$lib/labels/labels";
import { workspacePath } from "$lib/workspace/navigation";

export type ImportSource = components["schemas"]["ImportSource"];
export type ImportRun = components["schemas"]["ImportRun"];
export type ImportStatus = components["schemas"]["ImportStatus"];
export type ImportUnknownPolicy = components["schemas"]["ImportUnknownPolicy"];
export type ImportCatalogue = components["schemas"]["ImportCatalogue"];
export type ImportScope = components["schemas"]["ImportScope"];
export type ImportColumn = components["schemas"]["ImportColumn"];
export type ImportMapping = components["schemas"]["ImportMapping"];
export type ImportMappingKind = components["schemas"]["ImportMappingKind"];
export type ImportMappingPlan = components["schemas"]["ImportMappingPlan"];
export type ImportMappingDecision = components["schemas"]["ImportMappingDecision"];
export type ImportDecision = components["schemas"]["ImportDecision"];
export type ImportPreview = components["schemas"]["ImportPreview"];
export type ImportPreviewLine = components["schemas"]["ImportPreviewLine"];
export type ImportReport = components["schemas"]["ImportReport"];
export type ImportReportLine = components["schemas"]["ImportReportLine"];
export type ImportLedgerEntry = components["schemas"]["ImportLedgerEntry"];
export type ImportResource = components["schemas"]["ImportResource"];
export type ImportOutcome = components["schemas"]["ImportOutcome"];
export type ImportPhase = components["schemas"]["ImportPhase"];
export type ImportFile = components["schemas"]["ImportFile"];

type ConfigureResponses = operations["configureWorkspaceImport"]["responses"];
type CatalogueResponses = operations["getWorkspaceImportCatalogue"]["responses"];
type UploadResponses = operations["uploadWorkspaceImportFile"]["responses"];

export type ImportProblem =
	| ConfigureResponses[403 | 404 | 409 | 422 | 500 | 503]["content"]["application/problem+json"]
	| CatalogueResponses[429 | 502]["content"]["application/problem+json"]
	| UploadResponses[400 | 413]["content"]["application/problem+json"];

export const runPageSize = 25;
export const memberPageSize = 100;
export const pollIntervalMs = 4000;

export type ImportTargetOption = {
	id: string;
	name: string;
	detail?: string;
};

export type ImportTargets = {
	members: ImportTargetOption[];
	teams: ImportTargetOption[];
	projects: ImportTargetOption[];
	labels: ImportTargetOption[];
	states: ImportTargetOption[];
};

export const noTargets: ImportTargets = {
	members: [],
	teams: [],
	projects: [],
	labels: [],
	states: [],
};

export type ImportsView =
	| { kind: "loading" }
	| { kind: "sources"; sources: ImportSource[]; runs: ImportRun[]; nextCursor?: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type ImportRunView =
	| { kind: "loading" }
	| { kind: "connect"; run: ImportRun }
	| { kind: "catalogue"; run: ImportRun; catalogue: ImportCatalogue }
	| { kind: "staging"; run: ImportRun }
	| { kind: "rate_limited"; run: ImportRun; resumesAt?: string }
	| { kind: "staged"; run: ImportRun; plan: ImportMappingPlan; targets: ImportTargets }
	| { kind: "mapping"; run: ImportRun; plan: ImportMappingPlan; targets: ImportTargets }
	| { kind: "preview"; run: ImportRun; preview: ImportPreview }
	| { kind: "triage_ack"; run: ImportRun; preview: ImportPreview; teams: string[] }
	| { kind: "preview_stale"; run: ImportRun; preview: ImportPreview }
	| { kind: "executing"; run: ImportRun }
	| { kind: "imported"; run: ImportRun; report: ImportReport }
	| { kind: "reverting"; run: ImportRun }
	| { kind: "reverted"; run: ImportRun; report: ImportReport }
	| { kind: "failed"; run: ImportRun; report: ImportReport }
	| { kind: "source_refused"; run: ImportRun; reason?: string }
	| { kind: "encryption_unavailable"; run: ImportRun }
	| { kind: "not_found" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type ImportFailure =
	| { kind: "status_transition" }
	| { kind: "run_leased" }
	| { kind: "not_revertible" }
	| { kind: "preview_stale" }
	| { kind: "would_triage"; teams: string[] }
	| { kind: "source_refused"; reason?: string }
	| { kind: "rate_limited"; resumesAt?: string }
	| { kind: "encryption_unavailable" }
	| { kind: "unmapped" }
	| { kind: "team_mapping_required" }
	| { kind: "file_unreadable" }
	| { kind: "file_too_large" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function importFailure(problem: ImportProblem, resumesAt?: string): ImportFailure {
	if ("code" in problem) {
		switch (problem.code) {
			case "import_status_transition":
				return { kind: "status_transition" };
			case "import_run_leased":
				return { kind: "run_leased" };
			case "import_not_revertible":
				return { kind: "not_revertible" };
			case "import_preview_stale":
				return { kind: "preview_stale" };
			case "import_would_triage":
				return { kind: "would_triage", teams: problem.teams ?? [] };
			case "import_source_refused":
				return { kind: "source_refused", reason: problem.reason };
			case "import_signing_unavailable":
				return { kind: "encryption_unavailable" };
			case "rate_limited":
				return { kind: "rate_limited", resumesAt };
			case "forbidden":
				return { kind: "forbidden" };
		}
	}

	switch (problem.status) {
		case 400:
			return { kind: "file_unreadable" };
		case 403:
			return { kind: "team_mapping_required" };
		case 413:
			return { kind: "file_too_large" };
		case 422:
			return { kind: "unmapped" };
		default:
			return { kind: "unavailable" };
	}
}

export function failureTitle(failure: ImportFailure): string {
	switch (failure.kind) {
		case "would_triage":
			return "These teams put new issues in triage";
		case "preview_stale":
			return "This workspace changed while you were reading";
		case "rate_limited":
			return "The source is throttling this instance";
		case "source_refused":
			return "The source would not hand that over";
		case "encryption_unavailable":
			return "This instance cannot hold a source key";
		case "file_too_large":
			return "That file is too large";
		case "file_unreadable":
			return "That upload did not arrive as a file";
		case "forbidden":
			return "You may not run imports here";
		default:
			return "That did not work";
	}
}

export function failureMessage(failure: ImportFailure): string {
	switch (failure.kind) {
		case "status_transition":
			return "This run has already moved past that step. Reload to see where it stands.";
		case "run_leased":
			return "A worker is holding this run while it drains the source. Wait for it to finish, then try again.";
		case "not_revertible":
			return "Only a run that finished or failed can be taken back.";
		case "preview_stale":
			return "The preview below has been re-read. Check it again before importing.";
		case "would_triage":
			return `Anything imported into ${listed(failure.teams)} lands in triage rather than the backlog. Agree to that below, or change the triage setting on those teams first.`;
		case "source_refused":
			return failure.reason ?? "The source answered, but not with what this run asked for.";
		case "rate_limited":
			return failure.resumesAt
				? `The source has asked to be left alone until ${failure.resumesAt}. Nothing is lost; try again after that.`
				: "The source has asked to be left alone for a while. Nothing is lost; try again shortly.";
		case "encryption_unavailable":
			return "A source key is stored encrypted, and this instance has no encryption key to seal it with. An operator must set NORN_SECURITY_ENCRYPTION_KEY to 32 base64-encoded random bytes and restart the instance.";
		case "unmapped":
			return "Every source concept has to be decided before this run can go any further.";
		case "team_mapping_required":
			return "Only an administrator can let an import create a team or a project. Map each one onto a team you can already reach, or ask an administrator to run this import.";
		case "file_unreadable":
			return "Choose a file and upload it again.";
		case "file_too_large":
			return "Split it, or ask an operator what this instance accepts.";
		case "forbidden":
			return "Ask an administrator of this workspace.";
		case "unavailable":
			return "Check your connection and try again.";
	}
}

export function listed(names: string[]): string {
	if (names.length === 0) return "no teams";
	if (names.length === 1) return names[0];

	return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

const statusLabels: Record<ImportStatus, string> = {
	draft: "Not started",
	staging: "Reading the source",
	staged: "Read, waiting on decisions",
	mapped: "Ready to import",
	executing: "Importing",
	imported: "Imported",
	reverting: "Taking it back",
	reverted: "Taken back",
	failed: "Failed",
};

export function statusLabel(status: ImportStatus): string {
	return statusLabels[status] ?? status;
}

const statusTones: Record<ImportStatus, LabelColor> = {
	draft: "neutral",
	staging: "blue",
	staged: "blue",
	mapped: "violet",
	executing: "violet",
	imported: "cyan",
	reverting: "orchid",
	reverted: "orchid",
	failed: "magenta",
};

export function statusTone(status: ImportStatus): LabelColor {
	return statusTones[status] ?? "neutral";
}

export function settled(status: ImportStatus): boolean {
	return status === "imported" || status === "reverted" || status === "failed";
}

export function revertible(status: ImportStatus): boolean {
	return status === "imported" || status === "failed";
}

export function working(kind: ImportRunView["kind"]): boolean {
	return kind === "staging" || kind === "executing" || kind === "reverting";
}

const sourceNames: Record<string, string> = {
	linear: "Linear",
	csv: "CSV file",
};

export function sourceName(kind: string): string {
	return sourceNames[kind] ?? kind;
}

const sourceNotes: Record<string, string> = {
	linear:
		"Reads teams, states, labels, projects, cycles, issues, comments and uploaded files with a personal API key. The key is stored encrypted and never shown again.",
	csv: "Reads rows out of one file. You say which column is which, and one team stands in for the whole file.",
};

export function sourceNote(kind: string): string {
	return sourceNotes[kind] ?? "A source this instance carries.";
}

export const unknownPolicies: {
	value: ImportUnknownPolicy;
	label: string;
	note: string;
}[] = [
	{
		value: "skip",
		label: "Skip the row",
		note: "A row naming something that did not arrive is left behind and listed in the report.",
	},
	{
		value: "create",
		label: "Create what is missing",
		note: "A label or state a row names but the source never sent is created as the row is imported.",
	},
	{
		value: "fail",
		label: "Stop the import",
		note: "The first row naming something that did not arrive fails the run, so nothing else is applied.",
	},
];

const mappingKindHeadings: Record<ImportMappingKind, string> = {
	user: "People",
	state: "Statuses",
	priority: "Priorities",
	label: "Labels",
	project: "Projects",
	team: "Teams",
};

export function mappingKindHeading(kind: ImportMappingKind): string {
	return mappingKindHeadings[kind] ?? kind;
}

const mappingKindNotes: Record<ImportMappingKind, string> = {
	user: "Who wrote and who owns what arrives. Anyone left unattributed keeps their name in the issue body rather than an account.",
	state: "Where an imported issue sits on the board it lands on.",
	priority: "What the source called urgent, and what that means here.",
	label: "A label already here, or a new one created as the import runs.",
	project: "A project already here, or a new one created as the import runs.",
	team: "The team an imported issue belongs to. An issue whose team is skipped is skipped with it.",
};

export function mappingKindNote(kind: ImportMappingKind): string {
	return mappingKindNotes[kind] ?? "";
}

export const mappingKinds: ImportMappingKind[] = [
	"team",
	"state",
	"project",
	"label",
	"user",
	"priority",
];

export function decisionsFor(kind: ImportMappingKind): ImportDecision[] {
	switch (kind) {
		case "user":
			return ["map", "unattributed", "skip"];
		case "priority":
			return ["map", "skip"];
		default:
			return ["map", "create", "skip"];
	}
}

const decisionLabels: Record<ImportDecision, string> = {
	map: "Use one already here",
	create: "Create it",
	unattributed: "Leave unattributed",
	skip: "Skip it",
};

export function decisionLabel(decision: ImportDecision): string {
	return decisionLabels[decision] ?? decision;
}

export function targetsFor(kind: ImportMappingKind, targets: ImportTargets): ImportTargetOption[] {
	switch (kind) {
		case "user":
			return targets.members;
		case "team":
			return targets.teams;
		case "project":
			return targets.projects;
		case "label":
			return targets.labels;
		case "state":
			return targets.states;
		case "priority":
			return [];
	}
}

export function targetsByValue(kind: ImportMappingKind): boolean {
	return kind === "priority";
}

export function mappingLabel(mapping: ImportMapping): string {
	return mapping.sourceLabel?.trim() || mapping.sourceKey;
}

export function undecided(plan: ImportMappingPlan): ImportMapping[] {
	return plan.mappings.filter((mapping) => !mapping.decision);
}

export function undecidedOf(plan: ImportMappingPlan, kind: ImportMappingKind): ImportMapping[] {
	return undecided(plan).filter((mapping) => mapping.kind === kind);
}

export function suggestedOf(plan: ImportMappingPlan, kind: ImportMappingKind): ImportMapping[] {
	return undecidedOf(plan, kind).filter((mapping) => mapping.suggestedTargetId);
}

export function grouped(plan: ImportMappingPlan): {
	kind: ImportMappingKind;
	mappings: ImportMapping[];
}[] {
	return mappingKinds
		.map((kind) => ({
			kind,
			mappings: plan.mappings.filter((mapping) => mapping.kind === kind),
		}))
		.filter((group) => group.mappings.length > 0);
}

export function decisionOf(mapping: ImportMapping): ImportMappingDecision {
	return {
		kind: mapping.kind,
		sourceKey: mapping.sourceKey,
		decision: mapping.decision ?? "skip",
		targetId: mapping.targetId,
		targetValue: mapping.targetValue,
	};
}

const resourceLabels: Record<ImportResource, string> = {
	team: "Team",
	workflow_state: "Status",
	label_group: "Label group",
	label: "Label",
	project: "Project",
	cycle: "Cycle",
	issue: "Issue",
	issue_parent: "Sub-issue link",
	issue_relation: "Issue link",
	comment: "Comment",
	attachment: "File",
	embed: "Inline image",
};

export function resourceLabel(resource: ImportResource): string {
	return resourceLabels[resource] ?? resource;
}

const outcomeLabels: Record<ImportOutcome, string> = {
	created: "Created",
	skipped: "Skipped",
	unattributed: "Unattributed",
	adjusted: "Adjusted",
	dropped: "Dropped",
	failed: "Failed",
	deleted: "Deleted",
	archived: "Archived",
	retained: "Kept",
	skipped_modified: "Kept, edited since",
	skipped_in_use: "Kept, in use",
};

export function outcomeLabel(outcome: ImportOutcome): string {
	return outcomeLabels[outcome] ?? outcome;
}

const phaseLabels: Record<ImportPhase, string> = {
	execute: "Import",
	revert: "Revert",
};

export function phaseLabel(phase: ImportPhase): string {
	return phaseLabels[phase] ?? phase;
}

export type CsvColumnTarget = {
	value: string;
	label: string;
};

export const csvColumnTargets: CsvColumnTarget[] = [
	{ value: "ignore", label: "Do not import" },
	{ value: "title", label: "Title" },
	{ value: "description", label: "Description" },
	{ value: "assignee", label: "Assignee" },
	{ value: "state", label: "Status" },
	{ value: "labels", label: "Labels" },
	{ value: "priority", label: "Priority" },
	{ value: "estimate", label: "Estimate" },
	{ value: "due", label: "Due date" },
	{ value: "parent", label: "Parent" },
	{ value: "id", label: "Identifier in the source" },
	{ value: "created", label: "Created" },
];

export function csvColumnTargetLabel(value: string | undefined): string {
	return csvColumnTargets.find((target) => target.value === value)?.label ?? "Do not import";
}

const confidenceNotes: Record<string, string> = {
	certain: "The header says so",
	likely: "Read from the header",
	ambiguous: "Another column claims this too",
	none: "Nothing to read it from",
};

export function confidenceNote(confidence: string | undefined): string | undefined {
	return confidence ? (confidenceNotes[confidence] ?? confidence) : undefined;
}

export const csvDelimiters: { value: string; label: string }[] = [
	{ value: "", label: "Work it out from the file" },
	{ value: ",", label: "Comma" },
	{ value: ";", label: "Semicolon" },
	{ value: "\t", label: "Tab" },
	{ value: "|", label: "Pipe" },
];

export type CsvSettings = {
	objectKey: string;
	fileName: string;
	delimiter: string;
	header: boolean;
	teamKey: string;
	teamName: string;
	columns: { index: number; target: string }[];
};

export type LinearSettings = {
	teamIds: string[];
};

function held(run: ImportRun): Record<string, unknown> {
	return run.settings ?? {};
}

function text(value: unknown, fallback = ""): string {
	return typeof value === "string" ? value : fallback;
}

export function csvSettings(run: ImportRun): CsvSettings {
	const settings = held(run);
	const columns = Array.isArray(settings.columns) ? settings.columns : [];

	return {
		objectKey: text(settings.objectKey),
		fileName: text(settings.fileName),
		delimiter: text(settings.delimiter),
		header: settings.header !== false,
		teamKey: text(settings.teamKey),
		teamName: text(settings.teamName),
		columns: columns.flatMap((column) => {
			if (typeof column !== "object" || column === null) return [];

			const entry = column as Record<string, unknown>;

			return typeof entry.index === "number"
				? [{ index: entry.index, target: text(entry.target, "ignore") }]
				: [];
		}),
	};
}

export function linearSettings(run: ImportRun): LinearSettings {
	const settings = held(run);
	const teamIds = Array.isArray(settings.teamIds) ? settings.teamIds : [];

	return { teamIds: teamIds.filter((id): id is string => typeof id === "string") };
}

export function configured(run: ImportRun): boolean {
	return run.sourceKind === "csv" ? csvSettings(run).objectKey !== "" : run.sourceSecretSet;
}

export function counted(count: number, one: string, many: string): string {
	return `${count.toLocaleString("en-GB")} ${count === 1 ? one : many}`;
}

export function stagedSummary(run: ImportRun): string {
	return counted(run.staged, "record read", "records read");
}

export function processedSummary(run: ImportRun): string {
	return counted(run.processed, "record handled", "records handled");
}

export function previewLines(preview: ImportPreview): number {
	return (
		preview.created.length +
		preview.changed.length +
		preview.skipped.length +
		preview.unattributed.length
	);
}

export function importsPath(workspace: string): string {
	return workspacePath(workspace, "/settings/imports");
}

export function importPath(workspace: string, runId: string): string {
	return workspacePath(workspace, `/settings/imports/${runId}`);
}
