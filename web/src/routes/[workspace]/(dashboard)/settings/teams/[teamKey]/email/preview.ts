import type { IntakeSetting } from "$lib/triage/intake";

export type TeamEmailPreview = { intake: IntakeSetting };

export const teamEmailPreviewStates: Record<string, TeamEmailPreview> = import.meta.env.DEV
	? {
			intake_off: { intake: { kind: "off" } },
			intake_on: {
				intake: {
					kind: "on",
					address: {
						teamId: "00000000-0000-4000-8000-000000000101",
						email: "mobile-649848208d3e@submit.norn.so",
						localPart: "mobile-649848208d3e",
						domain: "submit.norn.so",
						createdAt: "2026-02-11T09:00:00Z",
					},
				},
			},
			intake_rotated: {
				intake: {
					kind: "on",
					address: {
						teamId: "00000000-0000-4000-8000-000000000101",
						email: "mobile-1f0c74a91b52@submit.norn.so",
						localPart: "mobile-1f0c74a91b52",
						domain: "submit.norn.so",
						createdAt: "2026-02-11T09:00:00Z",
						rotatedAt: "2026-03-02T11:30:00Z",
					},
				},
			},
			intake_unconfigured: { intake: { kind: "unconfigured" } },
			intake_unavailable: { intake: { kind: "unavailable" } },
		}
	: {};
