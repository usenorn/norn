import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

async function raiseLooseProject(page: Page, name: string, address: string) {
	await page.evaluate(
		async ([workspaceId, called, slug]) => {
			const answer = await fetch(`/v1/workspaces/${workspaceId}/projects`, {
				method: "POST",
				headers: { "content-type": "application/json" },
				body: JSON.stringify({ name: called, slug }),
			});

			if (!answer.ok) {
				throw new Error(`the project was refused: ${answer.status} ${await answer.text()}`);
			}
		},
		[fixture().workspaceId, name, address] as const
	);
}

test("a team's projects leave out the ones no team has claimed", async ({ page }) => {
	const stamp = Date.now();
	const loose = `Loose project ${stamp}`;

	await page.goto(at("/my-tasks"));
	await raiseLooseProject(page, loose, `loose-${stamp}`);

	await page.goto(at(`/teams/${fixture().teamKey}/projects`));

	await expect(page.getByRole("link", { name: /Billing/ }).first()).toBeVisible();
	await expect(page.getByText(loose)).toHaveCount(0);

	await page.goto(at("/projects"));

	await expect(page.getByRole("link", { name: /Billing/ }).first()).toBeVisible();
	await expect(page.getByText(loose)).toBeVisible();
});

test("the sidebar points a team at its own address, and marks only that entry", async ({ page }) => {
	await page.goto(at("/my-tasks"));

	const sidebar = page.getByRole("complementary");

	await sidebar.getByRole("link", { name: "Projects" }).nth(1).click();

	await expect(page).toHaveURL(new RegExp(`/teams/${fixture().teamKey}/projects$`));
	await expect(sidebar.getByRole("link", { name: "Projects" }).first()).not.toHaveAttribute(
		"aria-current",
		"page"
	);
});

test("an old link carrying a team in the query lands on that team's page", async ({ page }) => {
	await page.goto(at("/teams/" + fixture().teamKey + "/projects"));

	const teamId = await page.evaluate(
		async ([workspaceId, key]) => {
			const answer = await fetch(`/v1/workspaces/${workspaceId}/teams`);
			const teams = (await answer.json()) as { id: string; key: string }[];

			return teams.find((team) => team.key === key)?.id ?? "";
		},
		[fixture().workspaceId, fixture().teamKey] as const
	);

	await page.goto(at(`/projects?teamId=${teamId}&archived=0`));

	await expect(page).toHaveURL(
		new RegExp(`/teams/${fixture().teamKey}/projects\\?archived=0$`)
	);
});
