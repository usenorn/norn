export const keys = {
	page: (routeId: string | null) => `norn:page:${routeId ?? ""}`,
	workspaceScope: (workspaceId: string) => `norn:workspace:${workspaceId}`,
	issue: (issueId: string) => `norn:issue:${issueId}`,
	issues: (workspaceId: string) => `norn:issues:${workspaceId}`,
	inbox: (workspaceId: string) => `norn:inbox:${workspaceId}`,
	triage: (workspaceId: string) => `norn:triage:${workspaceId}`,
	members: (workspaceId: string) => `norn:members:${workspaceId}`,
	projects: (workspaceId: string) => `norn:projects:${workspaceId}`,
	cycles: (workspaceId: string) => `norn:cycles:${workspaceId}`,
	views: (workspaceId: string) => `norn:views:${workspaceId}`,
	account: () => "norn:account",
};
