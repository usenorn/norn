import { readFileSync } from "node:fs";
import {
	expect,
	request,
	test,
	type APIRequestContext,
	type APIResponse,
	type Page,
} from "@playwright/test";
import type { components } from "../src/lib/api/dashboard.gen";
import { fixture, statePath } from "./fixture";

type Roadmap = components["schemas"]["Roadmap"];
type RoadmapEntryContent = components["schemas"]["RoadmapEntryContent"];
type RoadmapVisibility = components["schemas"]["RoadmapVisibility"];
type Team = components["schemas"]["Team"];
type Project = components["schemas"]["Project"];
type Issue = components["schemas"]["Issue"];

const api = process.env.NORN_E2E_API ?? "http://localhost:8080";
const port = process.env.NORN_PREVIEW_PORT ?? "4173";
const appOrigin = `http://localhost:${port}`;
const loopback = `http://localhost:${port}`;
const roadmapsDomain = process.env.NORN_ROADMAPS_DOMAIN ?? "roadmap.localhost";

const roadmapHost = () => `${fixture().slug}.${roadmapsDomain}:${port}`;
const roadmapOrigin = () => `http://${roadmapHost()}`;

const secrets = {
	projectName: "SECRET-PROJECT-NAME-7f3a1c",
	projectDescription: "SECRET-PROJECT-DESCRIPTION-91c2e4",
	issueTitle: "SECRET-ISSUE-TITLE-0b8d55",
	issueDescription: "SECRET-ISSUE-DESCRIPTION-c47a19",
	looseTitle: "SECRET-LOOSE-TITLE-5e2f80",
	comment: "SECRET-COMMENT-BODY-4d1e6b",
	attachment: "SECRET-ATTACHMENT-b7e0a2.txt",
	person: "SECRET-PERSON-NAME-2a9f37",
};

const copy = {
	roadmapTitle: "Northwind public plan",
	roadmapDescription: "What Northwind customers can expect next.",
	projectTitle: "Faster checkout",
	projectDescription: "Pay in two taps with saved cards.",
	issueTitle: "Wallet payments",
	issueDescription: "Apple Pay and Google Pay on every device.",
	looseTitle: "Dark mode",
	looseDescription: "Follows the system setting.",
	timeline: "1 Oct 2026 – 31 Jan 2027",
	progress: "40% complete",
};

const everything: RoadmapVisibility = {
	issueLists: true,
	projectDescriptions: true,
	issueDescriptions: true,
	status: true,
	timeline: true,
	progress: true,
};

const flags = Object.keys(everything) as (keyof RoadmapVisibility)[];

function send(
	client: APIRequestContext,
	method: "GET" | "POST" | "PATCH" | "DELETE",
	path: string,
	body?: unknown
): Promise<APIResponse> {
	return client.fetch(`${api}/v1${path}`, {
		method,
		headers: { origin: appOrigin, referer: `${appOrigin}/`, "content-type": "application/json" },
		...(body === undefined ? {} : { data: body }),
	});
}

async function read<T>(answer: APIResponse, status = 200): Promise<T> {
	expect(answer.status(), await answer.text()).toBe(status);

	return (await answer.json()) as T;
}

function sessionCookie(): string {
	const state = JSON.parse(readFileSync(statePath, "utf8")) as {
		cookies: { name: string; value: string }[];
	};

	return state.cookies.map(({ name, value }) => `${name}=${value}`).join("; ");
}

async function served(
	client: APIRequestContext,
	path: string,
	headers: Record<string, string> = {}
): Promise<{ status: number; body: string; headers: Record<string, string> }> {
	const answer = await client.get(`${loopback}${path}`, {
		headers: { host: roadmapHost(), ...headers },
		maxRedirects: 0,
	});

	return { status: answer.status(), body: await answer.text(), headers: answer.headers() };
}

function dataPath(path: string): string {
	return `${path === "/" ? "" : path}/__data.json?x-sveltekit-invalidated=11`;
}

function expectNoSecrets(where: string, body: string) {
	for (const secret of Object.values(secrets)) {
		expect(body, `${where} leaked ${secret}`).not.toContain(secret);
	}
}

function teamKey(): string {
	return `P${Array.from(String(Date.now()).slice(-4), (digit) => "ABCDEFGHIJ"[Number(digit)]).join("")}`;
}

function captureResponses(page: Page): () => Promise<string> {
	const bodies: Promise<string>[] = [];

	page.on("response", (response) => {
		bodies.push(
			response
				.text()
				.catch(() => "")
				.then((text) => `${response.url()} ${JSON.stringify(response.headers())} ${text}`)
		);
	});

	return async () => (await Promise.all(bodies)).join("\n");
}

test.describe.configure({ mode: "serial" });

test.describe("the public roadmap page", () => {
	const workspace = () => `/workspaces/${fixture().workspaceId}`;

	let admin: APIRequestContext;
	let visitor: APIRequestContext;
	let team: Team;
	let project: Project;
	let issue: Issue;
	let loose: Issue;
	let projectId = "";
	let issueId = "";
	let looseId = "";

	async function roadmap(): Promise<Roadmap> {
		return read<Roadmap>(await send(admin, "GET", `${workspace()}/roadmap`));
	}

	async function publish(visibility: RoadmapVisibility) {
		const draft = await roadmap();

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: draft.version,
				title: copy.roadmapTitle,
				description: copy.roadmapDescription,
				visibility,
			})
		);

		await read(
			await send(admin, "POST", `${workspace()}/roadmap/publish`, {
				expectedVersion: (await roadmap()).version,
			})
		);
	}

	async function select(kind: "project" | "issue", sourceId: string, content: RoadmapEntryContent) {
		await read<Roadmap>(
			await send(admin, "POST", `${workspace()}/roadmap/entries`, {
				expectedVersion: (await roadmap()).version,
				kind,
				sourceId,
				content,
			}),
			201
		);
	}

	async function clear() {
		for (const entry of (await roadmap()).entries) {
			await read(
				await send(
					admin,
					"DELETE",
					`${workspace()}/roadmap/entries/${entry.id}?expectedVersion=${(await roadmap()).version}`
				)
			);
		}

		await read(await send(admin, "POST", `${workspace()}/roadmap/unpublish`));
	}

	test.beforeAll(async () => {
		admin = await request.newContext({ storageState: statePath });
		visitor = await request.newContext({ storageState: { cookies: [], origins: [] } });

		await clear();

		await read(await send(admin, "PATCH", "/accounts/me", { displayName: secrets.person }));

		const key = teamKey();

		team = await read<Team>(
			await send(admin, "POST", `${workspace()}/teams`, {
				key,
				name: `Public ${key}`,
				visibility: "public",
			}),
			201
		);

		project = await read<Project>(
			await send(admin, "POST", `${workspace()}/projects`, {
				slug: `public-${key.toLowerCase()}`,
				name: secrets.projectName,
				description: secrets.projectDescription,
				leadAccountId: fixture().accountId,
				teamIds: [team.id],
			}),
			201
		);

		issue = await read<Issue>(
			await send(admin, "POST", `${workspace()}/issues`, {
				teamId: team.id,
				projectId: project.id,
				title: secrets.issueTitle,
				description: secrets.issueDescription,
				assigneeId: fixture().accountId,
			}),
			201
		);

		loose = await read<Issue>(
			await send(admin, "POST", `${workspace()}/issues`, {
				teamId: team.id,
				title: secrets.looseTitle,
				assigneeId: fixture().accountId,
			}),
			201
		);

		await read(
			await send(admin, "POST", `${workspace()}/issues/${issue.id}/comments`, { body: secrets.comment }),
			201
		);

		await read(
			await send(admin, "POST", `${workspace()}/issues/${issue.id}/attachments`, {
				fileName: secrets.attachment,
				contentType: "text/plain",
				byteSize: 12,
			}),
			201
		);

		await select("project", project.id, {
			publicTitle: copy.projectTitle,
			publicDescription: copy.projectDescription,
			status: "in_progress",
			timeline: { startOn: "2026-10-01", endOn: "2027-01-31" },
			progressMode: "approved",
			progressPercent: 40,
			visibility: everything,
		});
		await select("issue", issue.id, {
			publicTitle: copy.issueTitle,
			publicDescription: copy.issueDescription,
			status: "completed",
			progressMode: "hidden",
			visibility: everything,
		});
		await select("issue", loose.id, {
			publicTitle: copy.looseTitle,
			publicDescription: copy.looseDescription,
			status: "paused",
			progressMode: "hidden",
			visibility: everything,
		});

		await publish(everything);

		const published = await read<components["schemas"]["PublicRoadmap"]>(
			await visitor.get(`${api}/v1/public/roadmap`, { headers: { host: roadmapHost() } })
		);

		projectId = published.projects[0].id;
		issueId = published.projects[0].issues?.[0].id ?? "";
		looseId = published.issues[0].id;

		expect(issueId).not.toBe("");
	});

	test.afterAll(async () => {
		await clear();
		await read(await send(admin, "PATCH", "/accounts/me", { displayName: fixture().displayName }));
		await admin.dispose();
		await visitor.dispose();
	});

	test("an anonymous visitor reads the roadmap and follows its detail links", async ({ browser }) => {
		const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		const page = await context.newPage();
		const bodies = captureResponses(page);

		await page.goto(`${roadmapOrigin()}/`);

		await expect(page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();
		await expect(page).toHaveTitle(copy.roadmapTitle);
		await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", "noindex");
		await expect(page.locator('meta[property="og:title"]')).toHaveAttribute("content", copy.roadmapTitle);
		await expect(page.locator('meta[property="og:description"]')).toHaveAttribute(
			"content",
			copy.roadmapDescription
		);
		await expect(page.getByText(copy.looseTitle)).toBeVisible();

		await page.evaluate(() => Reflect.set(window, "stayedOnPage", true));
		await page.getByRole("link", { name: new RegExp(copy.projectTitle) }).click();

		await expect(page).toHaveURL(`${roadmapOrigin()}/entries/${projectId}`);
		await expect(page.getByRole("heading", { level: 1, name: copy.projectTitle })).toBeVisible();
		expect(await page.evaluate(() => Reflect.get(window, "stayedOnPage"))).toBe(true);
		await expect(page).toHaveTitle(`${copy.projectTitle} · ${copy.roadmapTitle}`);
		await expect(page.getByText(copy.timeline)).toBeVisible();
		await expect(page.getByText(copy.progress)).toBeVisible();

		await page.getByRole("link", { name: new RegExp(copy.issueTitle) }).click();

		await expect(page).toHaveURL(`${roadmapOrigin()}/entries/${issueId}`);
		await expect(page.getByRole("heading", { level: 1, name: copy.issueTitle })).toBeVisible();
		await expect(page.getByText(copy.issueDescription)).toBeVisible();

		await page.getByRole("link", { name: copy.roadmapTitle }).click();

		await expect(page).toHaveURL(`${roadmapOrigin()}/`);
		await expect(page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();

		expectNoSecrets("the rendered page", await page.content());
		expectNoSecrets("a network response", await bodies());

		await context.close();
	});

	test("the keyboard reaches every detail and the page fits a narrow phone", async ({ browser }) => {
		const context = await browser.newContext({
			storageState: { cookies: [], origins: [] },
			viewport: { width: 360, height: 740 },
		});
		const page = await context.newPage();

		await page.goto(`${roadmapOrigin()}/`);
		await expect(page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();

		const projectLink = page.getByRole("link", { name: new RegExp(copy.projectTitle) });

		const focused = () => projectLink.evaluate((link) => link === document.activeElement);

		for (let press = 0; press < 10 && !(await focused()); press++) {
			await page.keyboard.press("Tab");
		}

		await expect(projectLink).toBeFocused();
		await page.keyboard.press("Enter");
		await expect(page.getByRole("heading", { level: 1, name: copy.projectTitle })).toBeVisible();

		for (const path of ["/", `/entries/${projectId}`]) {
			await page.goto(`${roadmapOrigin()}${path}`);

			const overflow = await page.evaluate(
				() => document.documentElement.scrollWidth - document.documentElement.clientWidth
			);

			expect(overflow, path).toBeLessThanOrEqual(0);
		}

		await context.close();
	});

	test("html and data are uncached, unindexed and identical for signed-in visitors", async () => {
		for (const path of ["/", `/entries/${projectId}`, `/entries/${issueId}`]) {
			for (const target of [path, dataPath(path)]) {
				const anonymous = await served(visitor, target);
				const signedIn = await served(visitor, target, { cookie: sessionCookie() });

				expect(anonymous.status, target).toBe(200);
				expect(anonymous.headers["cache-control"], target).toBe("no-store");
				expect(anonymous.headers["x-robots-tag"], target).toBe("noindex");
				expect(anonymous.body, target).not.toContain(fixture().email);
				expect(anonymous.body, target).not.toContain(fixture().workspaceId);
				expectNoSecrets(target, anonymous.body);
				expect(signedIn.body, target).toBe(anonymous.body);
			}
		}
	});

	test("every combination of visibility settings renders only what it allows", async () => {
		test.setTimeout(300_000);

		for (let mask = 0; mask < 2 ** flags.length; mask++) {
			const visibility = Object.fromEntries(
				flags.map((flag, bit) => [flag, Boolean(mask & (1 << bit))])
			) as RoadmapVisibility;
			const label = JSON.stringify(visibility);

			await publish(visibility);

			const overview = await served(visitor, "/");
			const overviewData = await served(visitor, dataPath("/"));
			const detail = await served(visitor, `/entries/${projectId}`);
			const detailData = await served(visitor, dataPath(`/entries/${projectId}`));
			const everywhere = [overview, overviewData, detail, detailData].map(({ body }) => body).join("\n");

			for (const answer of [overview, overviewData, detail, detailData]) {
				expect(answer.status, label).toBe(200);
				expect(answer.headers["cache-control"], label).toBe("no-store");
			}

			expect(overview.body, label).toContain(copy.projectTitle);
			expect(overview.body, label).toContain(copy.looseTitle);
			expectNoSecrets(label, everywhere);

			const shows = (body: string, text: string, shown: boolean) =>
				shown
					? expect(body, `${label} should show ${text}`).toContain(text)
					: expect(body, `${label} should hide ${text}`).not.toContain(text);

			shows(overview.body, copy.projectDescription, visibility.projectDescriptions);
			shows(overview.body, copy.timeline, visibility.timeline);
			shows(overview.body, copy.progress, visibility.progress);
			shows(overview.body, "In progress", visibility.status);
			shows(overview.body, "Paused", visibility.status);
			shows(overview.body, copy.looseDescription, visibility.issueDescriptions);

			shows(detail.body, `/entries/${issueId}`, visibility.issueLists);
			shows(detail.body, copy.issueTitle, visibility.issueLists);
			shows(detail.body, copy.issueDescription, visibility.issueLists && visibility.issueDescriptions);
			shows(overview.body, `/entries/${issueId}`, true);
			shows(overview.body, copy.issueDescription, visibility.issueDescriptions);

			if (!visibility.projectDescriptions) expect(everywhere, label).not.toContain(copy.projectDescription);
			if (!visibility.issueDescriptions) {
				expect(everywhere, label).not.toContain(copy.issueDescription);
				expect(everywhere, label).not.toContain(copy.looseDescription);
			}
			if (!visibility.timeline) expect(everywhere, label).not.toContain("2026-10-01");
			if (!visibility.status) expect(everywhere, label).not.toContain("in_progress");
		}

		await publish(everything);
	});

	test("unknown, unpublished and removed roadmaps and entries read the same", async ({ browser }) => {
		const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		const page = await context.newPage();
		const unavailable = page.getByRole("heading", { level: 1, name: "This roadmap is not available" });

		await page.goto(`${roadmapOrigin()}/entries/${issueId}`);
		await expect(page.getByRole("heading", { level: 1, name: copy.issueTitle })).toBeVisible();

		const draft = await roadmap();
		const entry = draft.entries.find((one) => one.issueId === issue.id);

		await read(
			await send(admin, "DELETE", `${workspace()}/roadmap/entries/${entry?.id}?expectedVersion=${draft.version}`)
		);
		await read(
			await send(admin, "POST", `${workspace()}/roadmap/publish`, { expectedVersion: (await roadmap()).version })
		);

		await page.reload();
		await expect(unavailable).toBeVisible();
		await expect(page).toHaveTitle("Roadmap unavailable");

		const removed = await served(visitor, dataPath(`/entries/${issueId}`));

		expect(removed.body).not.toContain(copy.issueTitle);
		expect(removed.headers["cache-control"]).toBe("no-store");

		for (const path of ["/", dataPath("/"), `/entries/${projectId}`, dataPath(`/entries/${projectId}`)]) {
			const after = await served(visitor, path);

			expect(after.body, path).not.toContain(copy.issueTitle);
			expect(after.body, path).not.toContain(copy.issueDescription);
			expect(after.body, path).not.toContain(issueId);
		}

		await page.goto(`${roadmapOrigin()}/entries/abcdefghijklmnop`);
		await expect(unavailable).toBeVisible();

		await page.goto(`http://nobody-${Date.now()}.${roadmapsDomain}:${port}/`);
		await expect(unavailable).toBeVisible();

		await page.goto(`${roadmapOrigin()}/`);
		await expect(page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();

		await read(await send(admin, "POST", `${workspace()}/roadmap/unpublish`));

		await page.reload();
		await expect(unavailable).toBeVisible();

		const gone = await served(visitor, dataPath("/"));

		expect(gone.body).not.toContain(copy.roadmapTitle);
		expect(gone.body).not.toContain(copy.projectTitle);

		await page.goto(`${roadmapOrigin()}/entries/${projectId}`);
		await expect(unavailable).toBeVisible();

		await context.close();
	});

	test("a roadmap host serves nothing but the roadmap", async ({ page }) => {
		expect((await served(visitor, "/sign-in")).status).toBe(404);
		expect((await served(visitor, `/${fixture().slug}`)).status).toBe(404);
		expect((await served(visitor, "/create-workspace")).status).toBe(404);

		await page.goto("/public-roadmap");
		await expect(page.getByRole("heading", { level: 1, name: "This roadmap is not available" })).toBeVisible();
		expect(await page.content()).not.toContain(fixture().email);
	});
});
