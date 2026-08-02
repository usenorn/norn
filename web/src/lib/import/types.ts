import type { Diagnostic } from "$lib/auth/types";

export type ImportSourceId = "jira" | "linear" | "github" | "csv";

export type MappingRow = {
	source: string;
	volume: string;
	value: string;
	options: string[];
	needsDecision: boolean;
};

export type MappingGroup = { name: string; rows: MappingRow[] };

export type PersonRow = MappingRow & { email: string };

export type FieldMapping = {
	groups: MappingGroup[];
	fieldsFound: number;
	fieldsMapped: number;
	affectedIssues: number;
};

export type PeopleMatching = {
	people: PersonRow[];
	matched: number;
};

export type ImportCountTone = "normal" | "warning" | "danger" | "muted";

export type ImportCount = { value: string; label: string; tone: ImportCountTone };

export type ImportDestination = { label: string; count: string };

export type ImportPlan = {
	counts: ImportCount[];
	destinations: ImportDestination[];
	excluded: string[];
};

export type ImportProgress = { percent: number; phase: string; meta: string };

export type ImportLogTone = "normal" | "warning" | "active";

export type ImportLogEntry = { at: string; message: string; tone: ImportLogTone };

export type SkippedGroup = { kind: string; reason: string; count: string };

export type ConnectFailure = { kind: "token_rejected"; diagnostics: Diagnostic[] };

export type ImportOutcome =
	| {
			kind: "complete";
			counts: ImportCount[];
			imported: string;
			landedIn: string;
			primaryTeam: string;
			finishedAt: string;
			duration: string;
	  }
	| {
			kind: "with_skips";
			counts: ImportCount[];
			imported: string;
			total: string;
			primaryTeam: string;
			skipped: SkippedGroup[];
			skippedTotal: string;
			finishedAt: string;
			duration: string;
	  }
	| {
			kind: "failed";
			counts: ImportCount[];
			stoppedAfter: string;
			resumeAt: string;
			total: string;
			stoppedAt: string;
	  };

export type ImportStage =
	| { kind: "choose_source" }
	| { kind: "connect"; failure: ConnectFailure | null }
	| { kind: "map_fields"; mapping: FieldMapping; unresolvedOnly: boolean }
	| { kind: "map_people"; matching: PeopleMatching }
	| { kind: "preview"; plan: ImportPlan }
	| {
			kind: "running";
			progress: ImportProgress;
			log: ImportLogEntry[];
			notifyEmail: string;
			detached: boolean;
	  }
	| { kind: "finished"; outcome: ImportOutcome };

export type ImportSession = { source: ImportSourceId; stage: ImportStage };
