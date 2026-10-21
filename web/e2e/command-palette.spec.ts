import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const issueActions = /^(Assign .+ to…|Change status of .+…|Move .+ to cycle…|Add label to .+…|Copy issue link)/;

async function openPalette(page: Page) {
	const palette = page.getByRole("dialog");

	await expect(async () => {
		await page.keyboard.press("ControlOrMeta+k");
		await expect(palette.getByRole("combobox")).toBeVisible({ timeout: 1_000 });
	}).toPass();

	return palette;
}

test("a team's issues are one search away and open at the team's own address", async ({ page }) => {
	const { slug, teamKey } = fixture();

	await page.goto(at("/my-tasks"));

	const palette = await openPalette(page);

	await palette.getByRole("combobox").fill("Billing");

	const teamIssues = palette.getByRole("option", { name: /^Issues\s*Billing/ });

	await expect(teamIssues).toBeVisible();
	await teamIssues.click();

	await expect(page).toHaveURL(new RegExp(`/${slug}/teams/${teamKey}/issues$`));
});

test("> from the inbox offers the global commands and nothing that needs an issue", async ({ page }) => {
	await page.goto(at("/inbox"));

	const palette = await openPalette(page);

	await palette.getByRole("combobox").fill(">");

	for (const name of ["New issue", "Open triage queue", "Toggle compact density"]) {
		await expect(palette.getByRole("option", { name: new RegExp(`^${name}`) })).toBeVisible();
	}

	await expect(palette.getByRole("option", { name: issueActions })).toHaveCount(0);
	await expect(palette.getByText(/commands act on/)).toHaveCount(0);
});

test("a selected issue gets its commands, and Escape backs out of the second step", async ({ page }) => {
	await page.goto(at(`/teams/${fixture().teamKey}/issues`));

	await expect(page.locator("[data-issue]").first()).toBeVisible();

	await page.keyboard.press("j");
	await page.keyboard.press("x");
	await expect(page.getByText("1 selected")).toBeVisible();

	const palette = await openPalette(page);
	const input = palette.getByRole("combobox");

	await input.fill(">assign");

	const assign = palette.getByRole("option", { name: /^Assign \S+ to…/ });

	await expect(assign).toBeVisible();
	await expect(palette.getByText(/commands act on \S+-\d+/)).toBeVisible();

	await input.press("Enter");

	const back = palette.getByRole("button", { name: /Assign \S+ to/ });

	await expect(back).toBeVisible();
	await expect(input).toHaveAttribute("placeholder", "Filter…");

	await input.press("Escape");

	await expect(palette).toBeVisible();
	await expect(back).toHaveCount(0);
	await expect(input).toHaveAttribute("placeholder", "Search issues, projects, people and views…");
});

test("a command shows its key only where the page handles it", async ({ page }) => {
	await page.goto(at(`/teams/${fixture().teamKey}/issues`));

	await expect(page.locator("[data-issue]").first()).toBeVisible();

	await expect(async () => {
		await page.keyboard.press("j");
		await expect(page.locator('[data-cursor="true"]')).toBeVisible({ timeout: 1_000 });
	}).toPass();

	await page.keyboard.press("x");
	await expect(page.getByText("1 selected")).toBeVisible();

	const selection = await openPalette(page);

	await selection.getByRole("combobox").fill(">assign");
	await expect(
		selection.getByRole("option", { name: /^Assign \S+ to…/ }).locator("kbd")
	).toHaveText("A");

	await page.keyboard.press("Escape");
	await page.goto(at(`/issues/${fixture().issues[0].reference}`));

	const open = await openPalette(page);

	await open.getByRole("combobox").fill(">assign");

	const assign = open.getByRole("option", { name: /^Assign \S+ to…/ });

	await expect(assign).toBeVisible();
	await expect(assign.locator("kbd")).toHaveCount(0);
});

test("the palette fits a 360px screen without scrolling sideways", async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 760 });
	await page.goto(at("/my-tasks"));

	await page.getByRole("button", { name: "Search" }).first().click();

	const palette = page.getByRole("dialog");

	await palette.getByRole("combobox").fill(">");
	await expect(palette.getByRole("option", { name: /^New issue/ })).toBeVisible();

	const fit = await palette.evaluate((dialog) => {
		const box = dialog.getBoundingClientRect();

		return {
			sideways: document.documentElement.scrollWidth > document.documentElement.clientWidth,
			left: box.left,
			right: box.right,
			width: document.documentElement.clientWidth,
		};
	});

	expect(fit.sideways).toBe(false);
	expect(fit.left).toBeGreaterThanOrEqual(0);
	expect(fit.right).toBeLessThanOrEqual(fit.width);
});
