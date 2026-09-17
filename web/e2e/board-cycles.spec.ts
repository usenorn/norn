import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;
const headers = { origin, referer: `${origin}/` };

async function billingTeam(page: Page): Promise<string> {
	const answer = await page.request.get(`/v1/workspaces/${fixture().workspaceId}/teams`);

	expect(answer.ok()).toBe(true);

	const teams = (await answer.json()) as { id: string; key: string }[];

	return teams.find((team) => team.key === fixture().teamKey)!.id;
}

async function runCycles(page: Page) {
	const team = await billingTeam(page);
	const answer = await page.request.put(
		`/v1/workspaces/${fixture().workspaceId}/teams/${team}/cycle-cadence`,
		{ headers, data: { lengthWeeks: 2, startsOn: 1 } }
	);

	expect(answer.ok()).toBe(true);
}

async function stopCycles(page: Page) {
	const { workspaceId, issues } = fixture();

	await page.request.post(`/v1/workspaces/${workspaceId}/issues/bulk`, {
		headers,
		data: { change: { clearCycle: true }, issueIds: issues.map((issue) => issue.id) },
	});

	const team = await billingTeam(page);

	await page.request.delete(`/v1/workspaces/${workspaceId}/teams/${team}/cycle-cadence`, {
		headers,
	});
}

async function openBoard(page: Page) {
	await page.goto(at(`/teams/${fixture().teamKey}/issues?layout=board`));
	await expect(page.locator('[data-cursor="true"]')).toBeVisible();
}

async function cursorCard(page: Page): Promise<string> {
	return (await page.locator('[data-cursor="true"]').getAttribute("data-issue")) ?? "";
}

async function pickCycle(page: Page): Promise<void> {
	await expect(page.getByRole("option", { name: "No cycle" })).toBeVisible();
	await page.getByRole("option").filter({ hasNotText: "No cycle" }).first().click();
}

async function openCycleNames(page: Page): Promise<string[]> {
	const team = await billingTeam(page);
	const answer = await page.request.get(
		`/v1/workspaces/${fixture().workspaceId}/cycles?teamId=${team}`
	);

	expect(answer.ok()).toBe(true);

	const cycles = (await answer.json()) as { name: string; closedAt?: string }[];

	return cycles.filter((cycle) => !cycle.closedAt).map((cycle) => cycle.name);
}

function teamCycles(url: URL): boolean {
	return url.pathname.endsWith("/cycles") && url.searchParams.has("teamId");
}

function card(page: Page, id: string) {
	return page.locator(`[data-issue="${id}"]`);
}

test.describe("cycles from the board", () => {
	test.afterEach(async ({ page }) => stopCycles(page));

	test("the card under the cursor moves to a cycle with Shift+C and shows it after a reload", async ({
		page,
	}) => {
		await runCycles(page);
		await openBoard(page);

		await page.keyboard.press("ArrowDown");

		const moved = await cursorCard(page);

		await page.keyboard.press("Shift+C");
		await expect(page.getByText("1 selected")).toBeVisible();

		await pickCycle(page);

		await expect(card(page, moved)).toContainText(/Cycle \d+/);

		await page.reload();

		await expect(card(page, moved)).toContainText(/Cycle \d+/);
	});

	test("every selected card moves to the cycle, and No cycle takes them out again", async ({
		page,
	}) => {
		await runCycles(page);
		await openBoard(page);

		const first = await cursorCard(page);

		await page.keyboard.press("x");
		await page.keyboard.press("ArrowDown");

		const second = await cursorCard(page);

		await page.keyboard.press("x");
		await expect(page.getByText("2 selected")).toBeVisible();

		await page.keyboard.press("Shift+C");
		await pickCycle(page);

		await expect(card(page, first)).toContainText(/Cycle \d+/);
		await expect(card(page, second)).toContainText(/Cycle \d+/);

		await page.keyboard.press("x");
		await expect(page.getByText("1 selected")).toBeVisible();

		await page.keyboard.press("Shift+C");
		await page.getByRole("option", { name: "No cycle" }).click();

		await expect(card(page, second)).not.toContainText(/Cycle \d+/);

		await page.reload();

		await expect(card(page, first)).toContainText(/Cycle \d+/);
		await expect(card(page, second)).not.toContainText(/Cycle \d+/);
	});

	test("Shift+C pressed while the team's cycles load offers every open cycle once they arrive", async ({
		page,
	}) => {
		await runCycles(page);

		const open = await openCycleNames(page);

		expect(open.length).toBeGreaterThanOrEqual(2);

		let release = () => {};
		const held = new Promise<void>((settle) => (release = settle));

		await page.route(teamCycles, async (route) => {
			await held;
			await route.continue();
		});

		await openBoard(page);
		await page.keyboard.press("Shift+C");

		await expect(page.getByText("Loading cycles…")).toBeVisible();
		await expect(page.getByRole("option")).toHaveCount(0);

		release();

		for (const name of [...open, "No cycle"]) {
			await expect(page.getByRole("option", { name, exact: true })).toBeVisible();
		}

		await expect(page.getByRole("option")).toHaveCount(open.length + 1);
	});

	test("cycles that cannot be read are reported instead of offering part of the list", async ({
		page,
	}) => {
		await runCycles(page);

		await page.route(teamCycles, (route) =>
			route.fulfill({
				status: 503,
				contentType: "application/problem+json",
				body: JSON.stringify({ title: "Unavailable", status: 503 }),
			})
		);

		await openBoard(page);
		await page.keyboard.press("Shift+C");

		await expect(page.getByText("Couldn’t load this team’s cycles")).toBeVisible();
		await expect(page.getByRole("option")).toHaveCount(0);
	});

	test("a team that runs no cycles is offered no cycle action", async ({ page }) => {
		await openBoard(page);

		await page.keyboard.press("Shift+C");
		await expect(page.getByText("1 selected")).toHaveCount(0);

		await page.keyboard.press("x");
		await expect(page.getByText("1 selected")).toBeVisible();
		await expect(page.getByRole("button", { name: "Move to cycle" })).toHaveCount(0);
	});

	test("an issue shows the cycle field only while its team runs cycles", async ({ page }) => {
		await page.goto(at(`/issues/${fixture().issues[0].reference}`));
		await expect(page.getByRole("button", { name: "Project: change" })).toBeVisible();
		await expect(page.getByRole("button", { name: "Cycle: change" })).toHaveCount(0);

		await runCycles(page);
		await page.reload();
		await expect(page.getByRole("button", { name: "Cycle: change" })).toBeVisible();
	});
});
