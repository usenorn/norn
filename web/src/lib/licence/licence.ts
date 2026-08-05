import type { components } from "$lib/api/dashboard.gen";

export type LicenceReport = components["schemas"]["LicenceReport"];
export type LicenceStatus = components["schemas"]["LicenceStatus"];
export type LicenceFeature = components["schemas"]["LicenceFeature"];

export type LicenceView =
	| { kind: "loading" }
	| { kind: "ready"; report: LicenceReport }
	| { kind: "unavailable" };

const statusLabels: Record<LicenceStatus, string> = {
	absent: "No licence",
	active: "Active",
	grace: "Expired, still running",
	expired: "Expired",
};

export function statusLabel(status: LicenceStatus): string {
	return statusLabels[status] ?? status;
}

const statusNotes: Record<LicenceStatus, string> = {
	absent:
		"This instance runs unlicensed, which is the normal state. Everything except the audit log and directory synchronization is free forever, on every tier, self-hosted included.",
	active: "Every feature this licence covers is available.",
	grace:
		"The licence has run out but its features keep working for now, so nothing breaks mid-flight. Nothing has been deleted and nothing will be.",
	expired:
		"The licence and its grace period have both run out, so the features it covered no longer accept new work. Nothing was deleted, and re-licensing restores them exactly as they were.",
};

export function statusNote(status: LicenceStatus): string {
	return statusNotes[status] ?? "";
}

const featureLabels: Record<string, string> = {
	audit: "Audit log",
	directory: "Directory synchronization",
};

export function featureLabel(name: string): string {
	return featureLabels[name] ?? name;
}

export const freeForever = [
	"Members, issues, projects, teams and agents — never counted, never capped",
	"Single sign-on through SAML or OpenID Connect",
	"The API, and every integration built on it",
	"Data export",
];
