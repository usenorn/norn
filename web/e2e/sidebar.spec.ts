import { expect, test } from "@playwright/test";
import { at } from "./fixture";

test("every row in the sidebar tree shares one band, and a team's pages sit one step in", async ({
	page,
}) => {
	await page.goto(at("/my-tasks"));

	const measured = await page
		.getByRole("complementary")
		.evaluate((side) => {
			const edge = side.getBoundingClientRect();
			const rows = [...side.querySelectorAll("[data-slot='sidebar-row']")].filter((row) =>
				/^(Inbox|My tasks|Billing|Issues|Projects)$/.test(row.textContent?.trim() ?? "")
			);

			return rows.map((row) => {
				const box = row.getBoundingClientRect();
				const link = row.matches("a") ? row : row.querySelector("a");
				const icon = link?.querySelector("svg")?.getBoundingClientRect();
				const label = [...(link?.querySelectorAll("span") ?? [])]
					.find((span) => span.textContent?.trim())
					?.getBoundingClientRect();

				return {
					label: row.textContent?.trim() ?? "",
					left: Math.round(box.left - edge.left),
					right: Math.round(edge.right - box.right),
					height: Math.round(box.height),
					icon: icon ? Math.round(icon.left - edge.left) : null,
					text: label ? Math.round(label.left - edge.left) : null,
				};
			});
		});

	expect(measured.length).toBeGreaterThan(3);
	expect(new Set(measured.map((row) => row.left)).size).toBe(1);
	expect(new Set(measured.map((row) => row.right)).size).toBe(1);
	expect(new Set(measured.map((row) => row.height)).size).toBe(1);

	const top = measured.find((row) => row.label === "Inbox");
	const team = measured.find((row) => row.label === "Billing");
	const child = measured.find((row) => row.label === "Issues");

	expect(team?.icon).toBe(top?.icon);
	expect(child?.icon).toBeGreaterThan(team?.icon ?? 0);
	expect(child?.icon).toBeLessThanOrEqual(team?.text ?? 0);
});

test("keyboard focus crosses a team row in the order the eye reads it", async ({ page }) => {
	await page.goto(at("/my-tasks"));

	await page.getByRole("link", { name: "Billing", exact: true }).focus();

	await page.keyboard.press("Tab");
	await expect(page.getByRole("button", { name: /^(Collapse|Expand) Billing$/ })).toBeFocused();

	await page.keyboard.press("Tab");
	await expect(page.getByRole("button", { name: "Actions for Billing" })).toBeFocused();
});

test("the expand button stands beside the team link, and a long name leaves both controls in view", async ({
	page,
}) => {
	await page.goto(at("/my-tasks"));

	const link = page.getByRole("link", { name: "Billing", exact: true });
	const toggle = page.getByRole("button", { name: /^(Collapse|Expand) Billing$/ });
	const actions = page.getByRole("button", { name: "Actions for Billing" });

	await expect(link.locator("button")).toHaveCount(0);
	expect(await toggle.evaluate((node) => node.closest("a") === null)).toBe(true);

	await link.evaluate((node) => {
		const label = [...node.querySelectorAll("span")].find(
			(span) => span.textContent?.trim() === "Billing"
		);

		if (label) label.textContent = "Billing, invoices, refunds and everything else about revenue";
	});

	const side = await page.getByRole("complementary").boundingBox();
	const name = await page
		.getByRole("complementary")
		.getByText(/^Billing, invoices/)
		.boundingBox();
	const toggleBox = await toggle.boundingBox();
	const actionsBox = await actions.boundingBox();

	await expect(toggle).toBeVisible();
	await expect(actions).toBeVisible();

	expect(name!.x + name!.width).toBeLessThanOrEqual(toggleBox!.x);
	expect(toggleBox!.x + toggleBox!.width).toBeLessThanOrEqual(actionsBox!.x);
	expect(actionsBox!.x + actionsBox!.width).toBeLessThanOrEqual(side!.x + side!.width);
});
