import type { AuditFilters, AuditRecord } from "$lib/audit/audit";

export type AuditPreview = {
	record?: AuditRecord;
	filters?: AuditFilters;
	busy?: boolean;
};

export const auditPreviewStates: Record<string, AuditPreview> = import.meta.env.DEV
	? {
			loading: { record: { kind: "loading" } },
			empty: { record: { kind: "empty", retentionDays: 365 } },
			ready: {
				record: {
					kind: "ready",
					retentionDays: 365,
					nextCursor: "preview-cursor",
					events: [
						{
							id: "00000000-0000-4000-8000-00000000a001",
							occurredAt: "2026-08-05T09:12:04Z",
							workspaceId: "00000000-0000-4000-8000-00000000a0f0",
							workspaceName: "Northwind",
							action: "sso.enforcement_changed",
							outcome: "succeeded",
							actor: {
								accountId: "00000000-0000-4000-8000-00000000a0a1",
								kind: "user",
								name: "Rae Whitfield",
								authMethod: "sso",
							},
							sourceIp: "203.0.113.24",
							userAgent: "Mozilla/5.0",
							resourceKind: "auth_policy",
							resourceName: "Northwind",
							detail: { enforcement: "sso" },
						},
						{
							id: "00000000-0000-4000-8000-00000000a002",
							occurredAt: "2026-08-05T08:57:41Z",
							workspaceId: "00000000-0000-4000-8000-00000000a0f0",
							workspaceName: "Northwind",
							action: "token.minted",
							outcome: "succeeded",
							actor: {
								accountId: "00000000-0000-4000-8000-00000000a0a2",
								kind: "token",
								name: "Priya Raman",
								tokenName: "deploy-bot",
								authMethod: "api_token",
							},
							sourceIp: "198.51.100.7",
							resourceKind: "api_token",
							resourceName: "deploy-bot",
						},
						{
							id: "00000000-0000-4000-8000-00000000a003",
							occurredAt: "2026-08-05T08:31:18Z",
							action: "session.sign_in_failed",
							outcome: "failed",
							actor: {
								kind: "user",
								authMethod: "password",
							},
							sourceIp: "198.51.100.99",
							resourceKind: "account",
							detail: { email: "rae@northwind.co" },
						},
						{
							id: "00000000-0000-4000-8000-00000000a004",
							occurredAt: "2026-08-05T07:02:55Z",
							workspaceId: "00000000-0000-4000-8000-00000000a0f0",
							workspaceName: "Northwind",
							action: "access.denied",
							outcome: "denied",
							actor: {
								accountId: "00000000-0000-4000-8000-00000000a0a3",
								kind: "agent",
								agentName: "triage-bot",
								authMethod: "agent",
							},
							resourceKind: "issue",
							detail: { reason: "role_lacks_action", action: "delete" },
						},
					],
				},
			},
			filtered_empty: {
				record: { kind: "empty", retentionDays: 365 },
				filters: {
					action: "workspace.purged",
					actor: "",
					resourceKind: "",
					from: "2026-01-01",
					to: "2026-02-01",
				},
			},
			unlicensed: {
				record: { kind: "unlicensed", retentionDays: 365, holder: "Northwind Studio" },
			},
			forbidden: { record: { kind: "forbidden" } },
			unavailable: { record: { kind: "unavailable" } },
			exporting: {
				record: { kind: "ready", retentionDays: 365, events: [] },
				busy: true,
			},
		}
	: {};
