import type { LicenceView } from "$lib/licence/licence";

export type LicencePreview = {
	view?: LicenceView;
};

export const licencePreviewStates: Record<string, LicencePreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			absent: {
				view: {
					kind: "ready",
					report: {
						status: "absent",
						features: [
							{ name: "audit", enabled: false },
							{ name: "directory", enabled: false },
						],
					},
				},
			},
			active: {
				view: {
					kind: "ready",
					report: {
						status: "active",
						holder: "Northwind Studio",
						issuedAt: "2026-01-14T09:00:00Z",
						expiresAt: "2027-01-14T09:00:00Z",
						graceEndsAt: "2027-02-13T09:00:00Z",
						features: [
							{ name: "audit", enabled: true },
							{ name: "directory", enabled: true },
						],
					},
				},
			},
			grace: {
				view: {
					kind: "ready",
					report: {
						status: "grace",
						holder: "Northwind Studio",
						issuedAt: "2025-08-01T09:00:00Z",
						expiresAt: "2026-08-01T09:00:00Z",
						graceEndsAt: "2026-08-31T09:00:00Z",
						features: [
							{ name: "audit", enabled: true },
							{ name: "directory", enabled: true },
						],
					},
				},
			},
			expired: {
				view: {
					kind: "ready",
					report: {
						status: "expired",
						holder: "Northwind Studio",
						issuedAt: "2024-06-01T09:00:00Z",
						expiresAt: "2025-06-01T09:00:00Z",
						graceEndsAt: "2025-07-01T09:00:00Z",
						features: [
							{ name: "audit", enabled: false },
							{ name: "directory", enabled: false },
						],
					},
				},
			},
			partial: {
				view: {
					kind: "ready",
					report: {
						status: "active",
						holder: "Meridian Labs",
						expiresAt: "2027-03-01T09:00:00Z",
						graceEndsAt: "2027-03-31T09:00:00Z",
						features: [
							{ name: "audit", enabled: true },
							{ name: "directory", enabled: false },
						],
					},
				},
			},
			unavailable: { view: { kind: "unavailable" } },
		}
	: {};
