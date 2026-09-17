import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;

function headers() {
	return { origin, referer: `${origin}/` };
}

async function seedLabel(
	page: Page,
	name: string,
	description: string
): Promise<{ id: string }> {
	const answer = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/labels`, {
		headers: headers(),
		data: { name, description, color: "cyan" },
	});

	expect(answer.ok()).toBe(true);

	return (await answer.json()) as { id: string };
}

async function applyLabel(page: Page, issueIndex: number, labelId: string): Promise<void> {
	const issue = fixture().issues[issueIndex];
	const read = await page.request.get(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issue.id}`,
		{ headers: headers() }
	);

	expect(read.ok()).toBe(true);

	const held = (await read.json()) as {
		labels: { id: string }[];
		fieldVersions?: Record<string, number>;
		version: number;
	};

	const answer = await page.request.put(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issue.id}/labels`,
		{
			headers: headers(),
			data: {
				labelIds: [...held.labels.map((label) => label.id), labelId],
				expectedVersion: held.fieldVersions?.labels ?? held.version,
			},
		}
	);

	expect(answer.ok()).toBe(true);
}

function rowAction(page: Page, name: string) {
	return page.getByRole("button", { name: `Actions for ${name}`, exact: true });
}

function row(page: Page, name: string) {
	return page.getByRole("listitem").filter({ has: rowAction(page, name) });
}

function everyRow(page: Page) {
	return page
		.getByRole("listitem")
		.filter({ has: page.getByRole("button", { name: /^Actions for / }) });
}

async function openRowMenu(page: Page, name: string) {
	await rowAction(page, name).click();
}

test.describe("the labels settings page", () => {
	test("a label created with a description is still described after a reload", async ({
		page,
	}) => {
		const name = `Bug ${Date.now()}`;
		const description = "Something shipped is wrong";

		await page.goto(at("/settings/labels"));

		await page.getByRole("button", { name: "New label" }).click();
		await page.getByPlaceholder("Label name").fill(name);
		await page.getByPlaceholder("What it is for (optional)").fill(description);
		await page.getByRole("button", { name: "Create", exact: true }).click();

		await expect(row(page, name)).toContainText(description);

		await page.reload();

		await expect(row(page, name)).toContainText(description);
	});

	test("the search box finds a label by its description, not only its name", async ({ page }) => {
		const stamp = Date.now();
		const name = `Plumbing ${stamp}`;
		const description = `deploy sync ${stamp}`;

		await seedLabel(page, name, description);
		await page.goto(at("/settings/labels"));

		await expect(row(page, name)).toBeVisible();

		await page.getByRole("textbox", { name: "Search labels" }).fill(`deploy sync ${stamp}`);

		await expect(row(page, name)).toBeVisible();
		await expect(everyRow(page)).toHaveCount(1);
	});

	test("an edited description replaces the old one and survives a reload", async ({ page }) => {
		const name = `Chore ${Date.now()}`;

		await seedLabel(page, name, "Necessary, not user-visible");
		await page.goto(at("/settings/labels"));

		await openRowMenu(page, name);
		await page.getByRole("menuitem", { name: "Edit label" }).click();

		await page.getByPlaceholder("What it is for (optional)").fill("Keeps the lights on");
		await page.getByRole("button", { name: "Save", exact: true }).click();

		await expect(row(page, name)).toContainText("Keeps the lights on");

		await page.reload();

		await expect(row(page, name)).toContainText("Keeps the lights on");
		await expect(row(page, name)).not.toContainText("Necessary, not user-visible");
	});

	test("a group created from the editor is selected at once and can be renamed", async ({
		page,
	}) => {
		const stamp = Date.now();
		const groupName = `Severity ${stamp}`;
		const renamed = `Impact ${stamp}`;
		const labelName = `Blocker ${stamp}`;

		await page.goto(at("/settings/labels"));

		await page.getByRole("button", { name: "New label" }).click();
		await page.getByPlaceholder("Label name").fill(labelName);
		await page.getByRole("button", { name: "Group", exact: true }).click();
		await page.getByRole("option", { name: "New group…" }).click();

		await page.getByRole("textbox", { name: "New group name" }).fill(groupName);
		await page.getByRole("button", { name: "Create group" }).click();

		await expect(page.getByRole("button", { name: "Group", exact: true })).toContainText(groupName);

		await page.getByRole("button", { name: "Create", exact: true }).click();

		await expect(row(page, labelName)).toBeVisible();
		await expect(page.getByRole("button", { name: `Actions for ${groupName}`, exact: true })).toBeVisible();

		await page.getByRole("button", { name: `Actions for ${groupName}`, exact: true }).click();
		await page.getByRole("menuitem", { name: "Rename group" }).click();
		await page.getByRole("textbox", { name: "Group name" }).fill(renamed);
		await page.getByRole("button", { name: "Save group name" }).click();

		await expect(
			page.getByRole("button", { name: `Actions for ${renamed}`, exact: true })
		).toBeVisible();

		await page.reload();

		await expect(
			page.getByRole("button", { name: `Actions for ${renamed}`, exact: true })
		).toBeVisible();
	});

	test("a merge moves the issues and leaves only the surviving label", async ({ page }) => {
		const stamp = Date.now();
		const source = `Spec ${stamp}`;
		const target = `Needs spec ${stamp}`;

		const created = await seedLabel(page, source, "Older duplicate");
		await seedLabel(page, target, "Blocked until product answers");
		await applyLabel(page, 0, created.id);

		await page.goto(at("/settings/labels"));

		await expect(row(page, source)).toContainText("1 issue");

		await openRowMenu(page, source);
		await page.getByRole("menuitem", { name: "Merge into another label" }).click();

		await page.getByRole("button", { name: "Merge into" }).click();
		await page.getByRole("option", { name: target, exact: true }).click();
		await page.getByRole("button", { name: "Merge labels" }).click();

		await expect(row(page, source)).toHaveCount(0);

		await page.reload();

		await expect(row(page, source)).toHaveCount(0);
		await expect(row(page, target)).toContainText("1 issue");
	});

	test("deleting names the count the API reports and removes the label", async ({ page }) => {
		const stamp = Date.now();
		const name = `Regression ${stamp}`;

		const created = await seedLabel(page, name, "Worked before, does not now");
		await applyLabel(page, 1, created.id);

		await page.goto(at("/settings/labels"));

		await openRowMenu(page, name);
		await page.getByRole("menuitem", { name: "Delete label" }).click();

		const dialog = page.getByRole("alertdialog");

		await expect(dialog).toContainText("It comes off 1 issue.");

		await dialog.getByRole("button", { name: "Delete anyway" }).click();

		await expect(row(page, name)).toHaveCount(0);

		await page.reload();

		await expect(row(page, name)).toHaveCount(0);
	});

	test("see tagged issues opens the issue list filtered to that label", async ({ page }) => {
		const stamp = Date.now();
		const name = `Infra ${stamp}`;

		const created = await seedLabel(page, name, "Build and deploy plumbing");
		await applyLabel(page, 2, created.id);

		await page.goto(at("/settings/labels"));

		await openRowMenu(page, name);
		await page.getByRole("menuitem", { name: "See tagged issues" }).click();

		await expect(page).toHaveURL(new RegExp(`/issues\\?label=${created.id}$`));
		await expect(page.getByText(fixture().issues[2].title)).toBeVisible();
	});
});
