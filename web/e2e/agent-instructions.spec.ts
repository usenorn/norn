import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;

function headers() {
	return { origin, referer: `${origin}/` };
}

async function registerAgent(page: Page, name: string): Promise<string> {
	const answer = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/agents`, {
		headers: headers(),
		data: {
			name,
			scopes: ["issue:read"],
			allTeams: true,
		},
	});

	expect(answer.ok()).toBe(true);

	const registered = (await answer.json()) as { agent: { id: string } };

	return registered.agent.id;
}

test("instructions written at every level are there after a reload, and can be emptied", async ({
	page,
}) => {
	const stamp = Date.now();
	const workspaceWritten = `Ship small. ${stamp}`;
	const projectWritten = `Touch the ledger only. ${stamp}`;
	const agentWritten = `Ask before deleting. ${stamp}`;

	const workspaceBox = page.getByLabel("Instructions for agents in this workspace");

	await page.goto(at("/settings"));
	await workspaceBox.fill(workspaceWritten);
	await page.getByRole("button", { name: "Save changes" }).click();
	await expect(page.getByText("Saved.")).toBeVisible();

	await page.goto(at("/settings"));
	await expect(workspaceBox).toHaveValue(workspaceWritten);

	const projectBox = page.getByLabel("Instructions for agents on this project");

	await page.goto(at("/projects/billing"));
	await page.getByRole("button", { name: "Edit" }).click();
	await projectBox.fill(projectWritten);
	await page.getByRole("button", { name: "Save changes" }).click();

	await page.goto(at("/projects/billing"));
	await page.getByRole("button", { name: "Edit" }).click();
	await expect(projectBox).toHaveValue(projectWritten);

	const agentId = await registerAgent(page, `instructed-${stamp}`);
	const agentBox = page.getByLabel("Instructions for this agent", { exact: true });

	const saveAgent = page.getByRole("button", { name: "Save instructions" });

	await page.goto(at(`/settings/agents/${agentId}`));
	await page.getByRole("tab", { name: "Instructions" }).click();
	await expect(saveAgent).toBeDisabled();

	await agentBox.fill(agentWritten);
	await expect(saveAgent).toBeEnabled();
	await saveAgent.click();
	await expect(saveAgent).toBeDisabled();

	await agentBox.fill(`${agentWritten} And say why.`);
	await expect(saveAgent).toBeEnabled();
	await saveAgent.click();
	await expect(saveAgent).toBeDisabled();

	await agentBox.fill(agentWritten);
	await expect(saveAgent).toBeEnabled();
	await saveAgent.click();
	await expect(saveAgent).toBeDisabled();

	await page.goto(at(`/settings/agents/${agentId}`));
	await page.getByRole("tab", { name: "Instructions" }).click();
	await expect(agentBox).toHaveValue(agentWritten);

	await page.goto(at("/settings"));
	await workspaceBox.fill("");
	await page.getByRole("button", { name: "Save changes" }).click();
	await expect(page.getByText("Saved.")).toBeVisible();

	await page.goto(at("/settings"));
	await expect(workspaceBox).toHaveValue("");
});

test("writing a project's instructions leaves the description it already had", async ({ page }) => {
	const stamp = Date.now();
	const described = "The shared reference nobody should lose.";

	await page.goto(at("/my-tasks"));

	const raised = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/projects`, {
		headers: headers(),
		data: { name: `Described ${stamp}`, slug: `described-${stamp}`, description: described },
	});

	expect(raised.ok()).toBe(true);

	const project = (await raised.json()) as { id: string };

	const written = await page.request.patch(
		`/v1/workspaces/${fixture().workspaceId}/projects/${project.id}`,
		{ headers: headers(), data: { agentInstructions: "Touch the ledger only." } }
	);

	expect(written.ok()).toBe(true);

	const read = await page.request.get(
		`/v1/workspaces/${fixture().workspaceId}/projects/${project.id}`,
		{ headers: headers() }
	);

	expect(read.ok()).toBe(true);

	const saved = (await read.json()) as { description: string; agentInstructions?: string };

	expect(saved.description).toBe(described);
	expect(saved.agentInstructions).toBe("Touch the ledger only.");

	const emptied = await page.request.patch(
		`/v1/workspaces/${fixture().workspaceId}/projects/${project.id}`,
		{ headers: headers(), data: { description: "" } }
	);

	expect(emptied.ok()).toBe(true);

	const after = await page.request.get(
		`/v1/workspaces/${fixture().workspaceId}/projects/${project.id}`,
		{ headers: headers() }
	);

	expect(((await after.json()) as { description: string }).description).toBe("");
});

test("a slow refresh after a save never puts the old instructions back", async ({ page }) => {
	const stamp = Date.now();
	const first = `Ask before deleting. ${stamp}`;
	const second = `Ask before deleting, and say why. ${stamp}`;

	const agentId = await registerAgent(page, `slow-refresh-${stamp}`);
	const box = page.getByLabel("Instructions for this agent", { exact: true });
	const save = page.getByRole("button", { name: "Save instructions" });

	await page.goto(at(`/settings/agents/${agentId}`));
	await page.getByRole("tab", { name: "Instructions" }).click();

	await box.fill(first);
	await save.click();
	await expect(save).toBeDisabled();

	let holdBack: (() => void) | undefined;
	const held = new Promise<void>((release) => {
		holdBack = release;
	});

	await page.route("**/__data.json*", async (route) => {
		await held;
		await route.continue();
	});

	await box.fill(second);
	await save.click();

	await expect(box).toHaveValue(second);
	await expect(save).toBeDisabled();

	holdBack?.();

	await expect(box).toHaveValue(second);
	await expect(save).toBeDisabled();

	await box.fill(first);
	await expect(save).toBeEnabled();
	await save.click();
	await expect(save).toBeDisabled();
	await expect(box).toHaveValue(first);

	await page.route(
		`**/v1/workspaces/*/agents/${agentId}/instructions`,
		async (route) => {
			await route.fulfill({
				status: 500,
				contentType: "application/problem+json",
				body: JSON.stringify({ status: 500, title: "Internal Server Error" }),
			});
		}
	);

	await box.fill(second);
	await save.click();

	await expect(page.getByText("Not saved")).toBeVisible();
	await expect(box).toHaveValue(second);
	await expect(save).toBeEnabled();

	await page.unroute(`**/v1/workspaces/*/agents/${agentId}/instructions`);

	await save.click();
	await expect(save).toBeDisabled();

	await page.goto(at(`/settings/agents/${agentId}`));
	await page.getByRole("tab", { name: "Instructions" }).click();
	await expect(box).toHaveValue(second);
});
