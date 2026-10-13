import { expect, request, test, type APIRequestContext, type APIResponse } from "@playwright/test";
import type { components } from "../src/lib/api/dashboard.gen";
import { fixture, ordinaryStatePath, statePath } from "./fixture";

type Roadmap = components["schemas"]["Roadmap"];
type RoadmapEntryContent = components["schemas"]["RoadmapEntryContent"];
type RoadmapVisibility = components["schemas"]["RoadmapVisibility"];
type PublicRoadmap = components["schemas"]["PublicRoadmap"];
type PublicRoadmapItem = components["schemas"]["PublicRoadmapItem"];
type Problem = components["schemas"]["RoadmapConflictProblem"];
type Team = components["schemas"]["Team"];
type Project = components["schemas"]["Project"];
type Issue = components["schemas"]["Issue"];
type WorkflowState = components["schemas"]["WorkflowState"];

const api = process.env.NORN_E2E_API ?? "http://localhost:8080";
const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;
const roadmapsDomain = process.env.NORN_ROADMAPS_DOMAIN ?? "roadmap.localhost";

const publicKeys = new Set([
	"title",
	"description",
	"publishedAt",
	"projects",
	"issues",
	"id",
	"kind",
	"status",
	"timeline",
	"startOn",
	"endOn",
	"progress",
]);

const everything: RoadmapVisibility = {
	issueLists: true,
	projectDescriptions: true,
	issueDescriptions: true,
	status: true,
	timeline: true,
	progress: true,
};

function send(
	client: APIRequestContext,
	method: "GET" | "POST" | "PATCH" | "DELETE",
	path: string,
	body?: unknown
): Promise<APIResponse> {
	return client.fetch(`${api}/v1${path}`, {
		method,
		headers: { origin, referer: `${origin}/`, "content-type": "application/json" },
		...(body === undefined ? {} : { data: body }),
	});
}

async function read<T>(answer: APIResponse, status = 200): Promise<T> {
	expect(answer.status(), await answer.text()).toBe(status);

	return (await answer.json()) as T;
}

function visit(client: APIRequestContext, path: string, host = `${fixture().slug}.${roadmapsDomain}`) {
	return client.get(`${api}/v1/public${path}`, { headers: { host } });
}

function content(publicTitle: string, extra: Partial<RoadmapEntryContent> = {}): RoadmapEntryContent {
	return {
		publicTitle,
		publicDescription: `${publicTitle}, as customers should read it`,
		progressMode: "hidden",
		visibility: everything,
		...extra,
	};
}

function unlistedKeys(value: unknown): string[] {
	if (Array.isArray(value)) return value.flatMap(unlistedKeys);
	if (value === null || typeof value !== "object") return [];

	return Object.entries(value).flatMap(([key, nested]) => [
		...(publicKeys.has(key) ? [] : [key]),
		...unlistedKeys(nested),
	]);
}

function teamKey(): string {
	return `R${Array.from(String(Date.now()).slice(-4), (digit) => "ABCDEFGHIJ"[Number(digit)]).join("")}`;
}

test.describe.configure({ mode: "serial" });

test.describe("a public roadmap", () => {
	const workspace = () => `/workspaces/${fixture().workspaceId}`;
	const internalTitles = {
		project: "Billing platform rewrite (do not share)",
		shipped: "Fix proration for ACME's contract",
		planned: "Retry webhooks for the Globex outage",
		secret: "Acquisition prep for Initech",
	};

	let admin: APIRequestContext;
	let member: APIRequestContext;
	let visitor: APIRequestContext;
	let team: Team;
	let project: Project;
	let shipped: Issue;
	let planned: Issue;
	let secret: Issue;
	let published: PublicRoadmap;

	async function roadmap(): Promise<Roadmap> {
		return read<Roadmap>(await send(admin, "GET", `${workspace()}/roadmap`));
	}

	async function complete(issue: Issue): Promise<Issue> {
		const states = await read<WorkflowState[]>(
			await send(admin, "GET", `${workspace()}/teams/${team.id}/states`)
		);
		const done = states.find((state) => state.isCompletion);

		expect(done, "the team has no completion state").toBeDefined();

		return read<Issue>(
			await send(admin, "PATCH", `${workspace()}/issues/${issue.id}`, {
				expectedVersion: issue.version,
				stateId: done?.id,
			})
		);
	}

	async function raise(title: string): Promise<Issue> {
		return read<Issue>(
			await send(admin, "POST", `${workspace()}/issues`, {
				teamId: team.id,
				projectId: project.id,
				title,
			}),
			201
		);
	}

	test.beforeAll(async () => {
		admin = await request.newContext({ storageState: statePath });
		member = await request.newContext({ storageState: ordinaryStatePath });
		visitor = await request.newContext();

		const leftover = await roadmap();

		for (const entry of leftover.entries) {
			const current = await roadmap();

			await read(
				await send(
					admin,
					"DELETE",
					`${workspace()}/roadmap/entries/${entry.id}?expectedVersion=${current.version}`
				)
			);
		}

		await read(await send(admin, "POST", `${workspace()}/roadmap/unpublish`));

		const key = teamKey();

		team = await read<Team>(
			await send(admin, "POST", `${workspace()}/teams`, {
				key,
				name: `Roadmap ${key}`,
				visibility: "public",
			}),
			201
		);

		project = await read<Project>(
			await send(admin, "POST", `${workspace()}/projects`, {
				slug: `roadmap-${key.toLowerCase()}`,
				name: internalTitles.project,
				teamIds: [team.id],
			}),
			201
		);

		shipped = await complete(await raise(internalTitles.shipped));
		planned = await raise(internalTitles.planned);
		secret = await raise(internalTitles.secret);
	});

	test.afterAll(async () => {
		await admin.dispose();
		await member.dispose();
		await visitor.dispose();
	});

	test("nothing is public before the first publication", async () => {
		expect((await visit(visitor, "/roadmap")).status()).toBe(404);
	});

	test("only an admin may read, select for or publish the roadmap", async () => {
		const { version } = await roadmap();

		expect((await send(member, "GET", `${workspace()}/roadmap`)).status()).toBe(403);
		expect(
			(
				await send(member, "POST", `${workspace()}/roadmap/entries`, {
					expectedVersion: version,
					kind: "project",
					sourceId: project.id,
					content: content("Faster billing"),
				})
			).status()
		).toBe(403);
		expect(
			(await send(member, "POST", `${workspace()}/roadmap/publish`, { expectedVersion: version })).status()
		).toBe(403);
	});

	test("an entry needs its own public title and a source this workspace can reach", async () => {
		const { version } = await roadmap();

		expect(
			(
				await send(admin, "POST", `${workspace()}/roadmap/entries`, {
					expectedVersion: version,
					kind: "issue",
					sourceId: planned.id,
					content: content("   "),
				})
			).status()
		).toBe(422);
		expect(
			(
				await send(admin, "POST", `${workspace()}/roadmap/entries`, {
					expectedVersion: version,
					kind: "issue",
					sourceId: crypto.randomUUID(),
					content: content("Guessed"),
				})
			).status()
		).toBe(404);
		expect(
			(
				await send(admin, "POST", `${workspace()}/roadmap/entries`, {
					expectedVersion: version,
					kind: "project",
					sourceId: fixture().workspaceId,
					content: content("Not a project"),
				})
			).status()
		).toBe(404);
	});

	test("a draft change made against an older version is refused with the current one", async () => {
		const { version } = await roadmap();

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: version,
				title: "Northwind roadmap",
				description: "What we are building next",
				visibility: everything,
			})
		);

		const conflict = await read<Problem>(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: version,
				title: "A competing edit",
				description: "",
				visibility: everything,
			}),
			409
		);

		expect(conflict.code).toBe("roadmap_stale");
		expect(conflict.version).toBe(version + 1);
		expect((await roadmap()).title).toBe("Northwind roadmap");
	});

	test("publishing serves only the reviewed public copy, uncached", async () => {
		const select = async (kind: "project" | "issue", sourceId: string, copy: RoadmapEntryContent) =>
			read<Roadmap>(
				await send(admin, "POST", `${workspace()}/roadmap/entries`, {
					expectedVersion: (await roadmap()).version,
					kind,
					sourceId,
					content: copy,
				}),
				201
			);

		await select("project", project.id, content("Faster billing", { progressMode: "calculated" }));
		await select("issue", shipped.id, content("Accurate proration"));
		await select("issue", planned.id, content("Reliable webhooks"));
		await complete(secret);

		const draft = await roadmap();

		await read(await send(admin, "POST", `${workspace()}/roadmap/publish`, { expectedVersion: draft.version }));

		const answer = await visit(visitor, "/roadmap");

		expect(answer.status(), await answer.text()).toBe(200);
		expect(answer.headers()["cache-control"]).toBe("no-store");

		published = (await answer.json()) as PublicRoadmap;

		expect(published.title).toBe("Northwind roadmap");
		expect(published.issues).toEqual([]);
		expect(published.projects).toHaveLength(1);

		const [faster] = published.projects;

		expect(faster.title).toBe("Faster billing");
		expect(faster.progress).toBe(50);
		expect(faster.issues?.map((issue) => issue.title).sort()).toEqual(["Accurate proration", "Reliable webhooks"]);
		expect(faster.issues?.find((issue) => issue.title === "Accurate proration")?.status).toBe("completed");
		expect(unlistedKeys(published)).toEqual([]);

		const ids = [faster, ...(faster.issues ?? [])].map((item) => item.id);

		for (const id of ids) {
			expect(id).toMatch(/^[a-z2-7]{16}$/);
		}

		const served = JSON.stringify(published);

		for (const leak of [
			...Object.values(internalTitles),
			team.id,
			team.key,
			project.id,
			shipped.id,
			shipped.reference,
			fixture().workspaceId,
			...draft.entries.map((entry) => entry.id),
		]) {
			expect(served).not.toContain(leak);
		}

		const one = await visit(visitor, `/roadmap/entries/${faster.issues?.[0].id}`);

		expect(one.status()).toBe(200);
		expect(one.headers()["cache-control"]).toBe("no-store");
		expect(((await one.json()) as PublicRoadmapItem).title).toBe(faster.issues?.[0].title);
	});

	test("internal edits and unpublished draft changes stay off the public roadmap", async () => {
		const current = await read<Issue>(await send(admin, "GET", `${workspace()}/issues/${planned.id}`));

		await read(
			await send(admin, "PATCH", `${workspace()}/issues/${planned.id}`, {
				expectedVersion: current.version,
				title: "Globex escalation: retry storm",
			})
		);

		const draft = await roadmap();
		const entry = draft.entries.find((one) => one.issueId === planned.id);

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap/entries/${entry?.id}`, {
				expectedVersion: draft.version,
				content: content("A title nobody has reviewed"),
			})
		);

		expect(await (await visit(visitor, "/roadmap")).json()).toEqual(published);
	});

	test("guessed identifiers and foreign hosts find nothing", async () => {
		const draft = await roadmap();
		const slug = fixture().slug;

		for (const guess of [draft.entries[0].id, shipped.id, shipped.reference, "aaaaaaaaaaaaaaaa"]) {
			expect((await visit(visitor, `/roadmap/entries/${guess}`)).status(), guess).toBe(404);
		}

		for (const host of [
			`nobody${Date.now()}.${roadmapsDomain}`,
			`${slug}.example.com`,
			roadmapsDomain,
			`deeper.${slug}.${roadmapsDomain}`,
		]) {
			expect((await visit(visitor, "/roadmap", host)).status(), host).toBe(404);
		}
	});

	test("the roadmap's visibility caps what any entry may show", async () => {
		const draft = await roadmap();

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: draft.version,
				title: draft.title,
				description: draft.description,
				visibility: { ...everything, projectDescriptions: false, issueDescriptions: false, progress: false },
			})
		);

		await read(
			await send(admin, "POST", `${workspace()}/roadmap/publish`, {
				expectedVersion: (await roadmap()).version,
			})
		);

		const capped = (await (await visit(visitor, "/roadmap")).json()) as PublicRoadmap;
		const [faster] = capped.projects;

		expect(faster.description).toBeUndefined();
		expect(faster.progress).toBeUndefined();
		expect(faster.issues?.length).toBe(2);

		for (const issue of faster.issues ?? []) {
			expect(issue.description).toBeUndefined();
		}
	});

	test("an issue whose team turns private drops out at once", async () => {
		const before = (await (await visit(visitor, "/roadmap")).json()) as PublicRoadmap;
		const hidden = before.projects[0].issues?.[0].id;

		await read(await send(admin, "PATCH", `${workspace()}/teams/${team.id}`, { visibility: "private" }));

		const after = (await (await visit(visitor, "/roadmap")).json()) as PublicRoadmap;

		expect(after.projects.map((item) => item.title)).toEqual(["Faster billing"]);
		expect(after.projects[0].issues).toEqual([]);
		expect(after.issues).toEqual([]);
		expect((await visit(visitor, `/roadmap/entries/${hidden}`)).status()).toBe(404);

		const draft = await roadmap();

		expect(draft.entries.filter((entry) => entry.withheld).map((entry) => entry.issueId).sort()).toEqual(
			[shipped.id, planned.id].sort()
		);
	});

	test("removing an entry or unpublishing takes it down at once", async () => {
		const draft = await roadmap();
		const entry = draft.entries.find((one) => one.kind === "project");
		const faster = ((await (await visit(visitor, "/roadmap")).json()) as PublicRoadmap).projects[0];

		await read(
			await send(
				admin,
				"DELETE",
				`${workspace()}/roadmap/entries/${entry?.id}?expectedVersion=${draft.version}`
			)
		);

		expect((await visit(visitor, `/roadmap/entries/${faster.id}`)).status()).toBe(404);
		expect(((await (await visit(visitor, "/roadmap")).json()) as PublicRoadmap).projects).toEqual([]);

		await read(await send(admin, "POST", `${workspace()}/roadmap/unpublish`));

		expect((await visit(visitor, "/roadmap")).status()).toBe(404);
	});
});
