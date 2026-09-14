import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test("a team's projects have an address of their own, and the workspace keeps its own list", async ({
	page,
}) => {
	await page.goto(at("/my-tasks"));

	const sidebar = page.getByRole("complementary");

	await sidebar.getByRole("link", { name: "Projects" }).nth(1).click();

	await expect(page).toHaveURL(new RegExp(`/teams/${fixture().teamKey}/projects$`));
	await expect(page.getByRole("link", { name: /Billing/ }).first()).toBeVisible();

	const workspaceProjects = sidebar.getByRole("link", { name: "Projects" }).first();

	await expect(workspaceProjects).not.toHaveAttribute("aria-current", "page");

	await workspaceProjects.click();

	await expect(page).toHaveURL(new RegExp(`${fixture().slug}/projects$`));
	await expect(page.getByRole("link", { name: /Billing/ }).first()).toBeVisible();
});
