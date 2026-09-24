import { expect, request, test, type APIRequestContext } from "@playwright/test";
import { fixture, ordinaryStatePath, statePath } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;

function headers() {
	return { origin, referer: `${origin}/` };
}

type Registered = { agent: { id: string; accountId: string; scope: string; projectId?: string } };

async function registerAgent(
	client: APIRequestContext,
	name: string,
	scope: { scope: string; projectId?: string }
): Promise<Registered["agent"]> {
	const answer = await client.post(`/v1/workspaces/${fixture().workspaceId}/agents`, {
		headers: headers(),
		data: { name, scopes: ["issue:read"], allTeams: true, ...scope },
	});

	expect(answer.status(), await answer.text()).toBe(201);

	return ((await answer.json()) as Registered).agent;
}

async function projectOf(client: APIRequestContext, slug: string): Promise<string> {
	const answer = await client.get(`/v1/workspaces/${fixture().workspaceId}/projects`, {
		headers: headers(),
	});
	const projects = (await answer.json()) as { id: string; slug: string }[];
	const found = projects.find((project) => project.slug === slug);

	expect(found, `no project addressed ${slug}`).toBeTruthy();

	return found!.id;
}

async function delegatable(client: APIRequestContext, issueId: string): Promise<string[]> {
	const answer = await client.get(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issueId}/delegation/agents`,
		{ headers: headers() }
	);

	expect(answer.ok(), await answer.text()).toBe(true);

	return ((await answer.json()) as { accountId: string }[]).map((agent) => agent.accountId);
}

test("a project agent refuses an issue outside its project, even to the person who registered it", async ({
	page,
}) => {
	const stamp = Date.now();
	const billing = await projectOf(page.request, "billing");
	const issue = fixture().issues[0];

	const loose = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/projects`, {
		headers: headers(),
		data: { slug: `loose-${stamp}`, name: `Loose ${stamp}` },
	});

	expect(loose.ok(), await loose.text()).toBe(true);

	const elsewhere = ((await loose.json()) as { id: string }).id;
	const agent = await registerAgent(page.request, `scoped-${stamp}`, {
		scope: "project",
		projectId: elsewhere,
	});

	expect(agent.scope).toBe("project");
	expect(agent.projectId).toBe(elsewhere);
	expect(await delegatable(page.request, issue.id)).not.toContain(agent.accountId);

	const refused = await page.request.post(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issue.id}/delegation`,
		{ headers: headers(), data: { agentAccountId: agent.accountId, brief: "" } }
	);

	expect(refused.status()).toBe(409);
	expect((await refused.json()) as { code: string }).toMatchObject({
		code: "issue_delegation_agent_not_yours",
	});

	const moved = await page.request.put(
		`/v1/workspaces/${fixture().workspaceId}/agents/${agent.id}/scope`,
		{ headers: headers(), data: { scope: "project", projectId: billing } }
	);

	expect(moved.ok(), await moved.text()).toBe(true);
	expect(await delegatable(page.request, issue.id)).toContain(agent.accountId);
});

test("a project nothing is scoped to deletes, and one an agent is scoped to says why it will not", async ({
	page,
}) => {
	const stamp = Date.now();
	const raised = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/projects`, {
		headers: headers(),
		data: { slug: `held-${stamp}`, name: `Held ${stamp}` },
	});

	expect(raised.ok(), await raised.text()).toBe(true);

	const held = ((await raised.json()) as { id: string }).id;
	const agent = await registerAgent(page.request, `held-${stamp}`, {
		scope: "project",
		projectId: held,
	});

	const refused = await page.request.delete(
		`/v1/workspaces/${fixture().workspaceId}/projects/${held}`,
		{ headers: headers() }
	);

	expect(refused.status()).toBe(409);
	expect((await refused.json()) as { code: string }).toMatchObject({
		code: "project_has_scoped_agents",
	});

	const narrowed = await page.request.put(
		`/v1/workspaces/${fixture().workspaceId}/agents/${agent.id}/scope`,
		{ headers: headers(), data: { scope: "member" } }
	);

	expect(narrowed.ok(), await narrowed.text()).toBe(true);

	const deleted = await page.request.delete(
		`/v1/workspaces/${fixture().workspaceId}/projects/${held}`,
		{ headers: headers() }
	);

	expect(deleted.ok(), await deleted.text()).toBe(true);
});

test.describe("somebody in the workspace who registered nothing", () => {
	test.use({ storageState: ordinaryStatePath });

	test("reaches an agent opened to the workspace and nothing narrower", async ({ page }) => {
		const stamp = Date.now();
		const owner = await request.newContext({
			storageState: statePath,
			baseURL: `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`,
		});
		const open = await registerAgent(owner, `open-${stamp}`, { scope: "workspace" });
		const kept = await registerAgent(owner, `kept-${stamp}`, { scope: "member" });

		await owner.dispose();

		const reachable = await delegatable(page.request, fixture().issues[0].id);

		expect(reachable).toContain(open.accountId);
		expect(reachable).not.toContain(kept.accountId);
	});

	test("may not open an agent to the whole workspace", async ({ page }) => {
		const refused = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/agents`, {
			headers: headers(),
			data: {
				name: `wide-${Date.now()}`,
				scopes: ["issue:read"],
				allTeams: true,
				scope: "workspace",
			},
		});

		expect(refused.status()).toBe(409);
		expect((await refused.json()) as { code: string }).toMatchObject({
			code: "agent_scope_forbidden",
		});
	});
});
