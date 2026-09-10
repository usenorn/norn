import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test.describe("My tasks", () => {
	test("the header controls all lead somewhere", async ({ page }) => {
		await page.goto(at("/my-tasks"));

		await expect(page.getByRole("heading", { name: "My tasks" })).toBeVisible();

		await page.getByRole("link", { name: "Notifications" }).click();
		await expect(page).toHaveURL(new RegExp(`${fixture().slug}/inbox$`));

		await page.goBack();
		await page.getByRole("button", { name: "New task" }).click();

		const dialog = page.getByRole("dialog");

		await expect(dialog.getByRole("button", { name: fixture().displayName })).toBeVisible();

		await dialog.getByRole("button", { name: "Cancel" }).click();
	});

	test("filtering narrows the list and says so when nothing matches", async ({ page }) => {
		await page.goto(at("/my-tasks"));

		await page.getByRole("button", { name: "Filter" }).click();
		await page.getByRole("option", { name: "Due" }).click();
		await page.getByRole("option", { name: "Overdue" }).click();

		await expect(page.getByText("Due: Overdue")).toBeVisible();
		await expect(page.getByRole("link", { name: /Proration is off by one day/ })).toBeVisible();
		await expect(page.getByRole("link", { name: /Audit log export times out/ })).toHaveCount(0);

		await page.goto(at("/my-tasks?due=today&priority=low"));

		await expect(page.getByText("No tasks match these filters")).toBeVisible();

		await page.getByRole("link", { name: "Clear filters" }).click();

		await expect(page.getByRole("link", { name: /Audit log export times out/ })).toBeVisible();
	});

	test("the grouping chosen in display options is remembered", async ({ page }) => {
		await page.goto(at("/my-tasks"));

		await expect(page.getByRole("button", { name: /Grouped by due date/ })).toBeVisible();

		await page.getByRole("button", { name: "Display options" }).click();
		await page.getByRole("button", { name: /Grouping/ }).click();
		await page.getByRole("link", { name: "Status", exact: true }).click();

		await expect(page.getByRole("button", { name: /Grouped by status/ })).toBeVisible();

		await page.goto(at("/my-tasks"));

		await expect(page.getByRole("button", { name: /Grouped by status/ })).toBeVisible();

		await page.getByRole("button", { name: "Display options" }).click();
		await page.getByRole("link", { name: "Reset display" }).click();

		await expect(page.getByRole("button", { name: /Grouped by due date/ })).toBeVisible();
	});
});
