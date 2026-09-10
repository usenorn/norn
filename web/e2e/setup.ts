import { mkdir, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import { request, type APIRequestContext } from "@playwright/test";
import { fixturePath, statePath, type Fixture } from "./fixture";

const api = process.env.NORN_E2E_API ?? "http://localhost:8080";
const mail = process.env.NORN_E2E_MAILPIT ?? "http://localhost:8025";
const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;
const password = "Sequoia-Winter-2026";

async function call<T>(
	client: APIRequestContext,
	method: "get" | "post",
	path: string,
	body?: unknown
): Promise<T> {
	const answer = await client[method](`${api}/v1${path}`, {
		headers: { origin, referer: `${origin}/`, "content-type": "application/json" },
		...(body === undefined ? {} : { data: body }),
	});

	if (!answer.ok()) {
		throw new Error(`${method.toUpperCase()} ${path} answered ${answer.status()}: ${await answer.text()}`);
	}

	return (await answer.json()) as T;
}

async function confirmationToken(client: APIRequestContext, email: string): Promise<string> {
	for (let asked = 0; asked < 40; asked += 1) {
		const box = await client.get(`${mail}/api/v1/messages?limit=30`);
		const listing = (await box.json()) as { messages: { ID: string; To: { Address: string }[] }[] };
		const held = listing.messages.find((message) =>
			message.To.some((to) => to.Address.toLowerCase() === email)
		);

		if (held) {
			const body = await client.get(`${mail}/api/v1/message/${held.ID}`);
			const found = /sign-up\/confirm\?token=([A-Za-z0-9._-]+)/.exec(await body.text());

			if (found) return found[1];
		}

		await new Promise((settle) => setTimeout(settle, 500));
	}

	throw new Error(`no confirmation link arrived for ${email}`);
}

function due(days: number): string {
	const day = new Date();

	day.setUTCDate(day.getUTCDate() + days);

	return day.toISOString().slice(0, 10);
}

export default async function setup(): Promise<void> {
	const client = await request.newContext();
	const stamp = Date.now();
	const email = `e2e${stamp}@example.test`;

	await call(client, "post", "/auth/sign-up", {
		email,
		displayName: "Rae Okafor",
		password,
		timezone: "UTC",
	});

	const token = await confirmationToken(client, email);
	const confirmed = await call<{ account: { id: string } }>(client, "post", "/auth/sign-up/confirm", {
		token,
	});

	const account = confirmed.account.id;
	const workspace = await call<{ id: string; slug: string }>(client, "post", "/workspaces", {
		slug: `e2e${stamp}`,
		name: "Northwind",
		team: { key: "BIL", name: "Billing" },
	});

	const teams = await call<{ id: string }[]>(client, "get", `/workspaces/${workspace.id}/teams`);
	const team = teams[0].id;

	const bug = await call<{ id: string }>(client, "post", `/workspaces/${workspace.id}/labels`, {
		name: "Bug",
		color: "magenta",
	});

	const project = await call<{ id: string }>(client, "post", `/workspaces/${workspace.id}/projects`, {
		slug: "billing",
		name: "Billing",
		teamIds: [team],
	});

	const raised = [
		{ title: "Proration is off by one day on annual plans", priority: "urgent", dueOn: due(-2) },
		{ title: "Offline queue drops edits on reconnect", priority: "high", dueOn: due(0) },
		{ title: "Invoice PDF misses the tax line", priority: "medium", dueOn: due(1) },
		{ title: "Retry webhook deliveries with backoff", priority: "high", dueOn: due(20) },
		{ title: "Audit log export times out", priority: "low" },
	];

	const issues: Fixture["issues"] = [];

	for (const one of raised) {
		const issue = await call<{ id: string; reference: string }>(
			client,
			"post",
			`/workspaces/${workspace.id}/issues`,
			{ teamId: team, assigneeId: account, labelIds: [bug.id], projectId: project.id, ...one }
		);

		issues.push({ id: issue.id, reference: issue.reference, title: one.title });
	}

	const fixture: Fixture = {
		workspaceId: workspace.id,
		slug: workspace.slug,
		teamKey: "BIL",
		accountId: account,
		displayName: "Rae Okafor",
		email,
		issues,
	};

	await mkdir(dirname(statePath), { recursive: true });
	await client.storageState({ path: statePath });
	await writeFile(fixturePath, JSON.stringify(fixture, null, "\t"));
	await client.dispose();
}
