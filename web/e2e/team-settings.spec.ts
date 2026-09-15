import { expect, test } from "@playwright/test";
import { at, fixture, ordinaryStatePath } from "./fixture";

const sections = [
	["general", "General"],
	["members", "Members"],
	["notifications", "Notifications"],
	["states", "States"],
	["templates", "Issue templates"],
	["cycles", "Cycles"],
	["triage", "Triage"],
	["email", "Issues by email"],
	["source-control", "Source control"],
	["agents", "Agents"],
] as const;

function settingsAt(section?: string): string {
	return at(`/settings/teams/${fixture().teamKey}${section ? `/${section}` : ""}`);
}

test("a team's settings open on its sections, grouped, each linking to its own page", async ({
	page,
}) => {
	await page.goto(settingsAt());

	const list = page.getByRole("navigation", { name: "Team settings" });

	for (const group of ["Team", "Work", "Intake & automation"]) {
		await expect(list.getByRole("heading", { name: group, exact: true })).toBeVisible();
	}

	for (const [section, title] of sections) {
		await expect(list.getByRole("link", { name: new RegExp(`^${title}`) })).toHaveAttribute(
			"href",
			new RegExp(`/settings/teams/${fixture().teamKey}/${section}$`)
		);
	}

	await list.getByRole("link", { name: /^Cycles/ }).click();

	await expect(page).toHaveURL(new RegExp(`/settings/teams/${fixture().teamKey}/cycles$`));
	await expect(page.getByRole("heading", { level: 1, name: "Cycles" })).toBeVisible();
});

test("every section opens at its own address and comes back the same after a reload", async ({
	page,
}) => {
	for (const [section, title] of sections) {
		await page.goto(settingsAt(section));
		await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();

		await page.reload();
		await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
	}
});

test("a state is added from a dialog, and closing a dialog puts focus back on the button that opened it", async ({
	page,
}) => {
	const name = `Checking ${Date.now()}`;

	await page.goto(settingsAt("states"));

	const open = page.getByRole("button", { name: "Add a state" });

	await open.click();

	const dialog = page.getByRole("dialog", { name: "Add a state" });

	await expect(dialog).toBeVisible();
	await expect(dialog.getByLabel("Name")).toBeFocused();

	await dialog.getByLabel("Name").fill(name);
	await dialog.getByRole("button", { name: "Add state" }).click();

	await expect(page.getByRole("dialog")).toHaveCount(0);
	await expect(page.getByRole("list").getByText(name, { exact: true })).toBeVisible();
	await expect(open).toBeFocused();

	await open.click();
	await expect(page.getByRole("dialog")).toBeVisible();

	await page.keyboard.press("Escape");

	await expect(page.getByRole("dialog")).toHaveCount(0);
	await expect(open).toBeFocused();
});

test("someone is looked for from a dialog, and Escape puts focus back on its button", async ({
	page,
}) => {
	await page.goto(settingsAt("members"));

	const open = page.getByRole("button", { name: "Add someone" });

	await open.click();

	const dialog = page.getByRole("dialog", { name: /^Add someone to / });

	await expect(dialog).toBeVisible();
	await expect(dialog.getByRole("searchbox")).toBeFocused();

	await dialog.getByRole("searchbox").fill(fixture().ordinary.displayName);
	await expect(dialog.getByText(/^Nobody in .+ matches/)).toBeVisible();

	await page.keyboard.press("Escape");

	await expect(page.getByRole("dialog")).toHaveCount(0);
	await expect(open).toBeFocused();
});

test("the cycles page sends its settings links straight to the cycles section", async ({
	page,
}) => {
	await page.goto(at(`/cycles/${fixture().teamKey}`));

	await expect(page.getByRole("link", { name: /^(Turn on cycles|Change cadence)$/ })).toHaveAttribute(
		"href",
		new RegExp(`/settings/teams/${fixture().teamKey}/cycles$`)
	);
});

test.describe("somebody who is on the team but does not run the workspace", () => {
	test.use({ storageState: ordinaryStatePath });

	test("can read a team's settings but is offered nothing the server would refuse", async ({
		page,
	}) => {
		await page.goto(settingsAt("states"));

		await expect(page.getByText("You cannot change this team")).toBeVisible();
		await expect(page.getByRole("button", { name: "Add a state" })).toBeDisabled();

		await page.goto(settingsAt("members"));

		await expect(page.getByRole("heading", { level: 1, name: "Members" })).toBeVisible();
		await expect(page.getByRole("button", { name: "Add someone" })).toHaveCount(0);
	});
});
