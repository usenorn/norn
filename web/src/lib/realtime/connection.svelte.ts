import { getContext, setContext } from "svelte";
import { invalidate, invalidateAll } from "$app/navigation";
import { keys } from "$lib/api/keys";
import { sessionParam } from "$lib/account/accounts";
import type { components } from "$lib/api/dashboard.gen";

export type Issue = components["schemas"]["Issue"];
export type IssueComment = components["schemas"]["IssueComment"];

export type RealtimeState = "connecting" | "live" | "reconnecting" | "stale" | "off";

export type RealtimeEventKind =
	| "issue.created"
	| "issue.updated"
	| "issue.deleted"
	| "comment.posted"
	| "comment.edited"
	| "comment.deleted"
	| "notification.arrived"
	| "membership.changed"
	| "execution.updated"
	| "execution.event"
	| "execution.changeset"
	| "question.asked"
	| "question.settled";

export type RealtimeEvent = {
	kind: RealtimeEventKind;
	issueId: string;
	teamId: string;
	actorId: string;
	subject: string;
	payload: unknown;
};

export type RealtimeHandler = (event: RealtimeEvent) => void;

export const staleAfterMs = 30_000;

export const connectTimeoutMs = 15_000;

export const refetchWindowMs = 400;

const key = Symbol("norn.realtime");

export class RealtimeConnection {
	state = $state<RealtimeState>("connecting");

	#source: EventSource | null = null;
	#handlers = new Set<RealtimeHandler>();
	#staleTimer: ReturnType<typeof setTimeout> | undefined;
	#connectTimer: ReturnType<typeof setTimeout> | undefined;
	#refetchTimer: ReturnType<typeof setTimeout> | undefined;
	#pending = new Set<string>();
	#workspaceId = "";
	#slot = "";
	#topics = "workspace,inbox";

	get connected(): boolean {
		return this.state === "live";
	}

	get degraded(): boolean {
		return this.state === "stale" || this.state === "off";
	}

	open(workspaceId: string, slot: string, topics = "workspace,inbox") {
		if (typeof EventSource === "undefined") {
			this.state = "off";

			return;
		}

		if (
			this.#source &&
			this.#workspaceId === workspaceId &&
			this.#slot === slot &&
			this.#topics === topics
		) {
			return;
		}

		this.close();

		this.#workspaceId = workspaceId;
		this.#slot = slot;
		this.#topics = topics;
		this.state = "connecting";

		this.#connect();
	}

	#connect() {
		const query = new URLSearchParams({ topics: this.#topics });

		if (this.#slot) query.set(sessionParam, this.#slot);

		const source = new EventSource(`/v1/workspaces/${this.#workspaceId}/events?${query}`);

		this.#connectTimer = setTimeout(() => this.#reconnect(), connectTimeoutMs);

		source.onopen = () => {
			clearTimeout(this.#connectTimer);
			clearTimeout(this.#staleTimer);
			this.#staleTimer = undefined;
			this.state = "live";
		};

		source.onerror = () => {
			clearTimeout(this.#connectTimer);

			if (source.readyState === EventSource.CLOSED) {
				this.state = "off";
				clearTimeout(this.#staleTimer);
				this.#staleTimer = undefined;

				return;
			}

			this.#degrade();
		};

		source.addEventListener("resync", () => void invalidateAll());

		for (const kind of eventKinds) {
			source.addEventListener(kind, (message) => this.#dispatch(kind, message as MessageEvent));
		}

		this.#source = source;
	}

	#reconnect() {
		this.#source?.close();
		this.#source = null;
		this.#degrade();
		this.#connect();
	}

	#degrade() {
		if (this.state !== "stale") this.state = "reconnecting";

		if (this.#staleTimer !== undefined) return;

		this.#staleTimer = setTimeout(() => {
			this.state = "stale";
			this.#staleTimer = undefined;
		}, staleAfterMs);
	}

	close() {
		clearTimeout(this.#staleTimer);
		clearTimeout(this.#connectTimer);
		clearTimeout(this.#refetchTimer);
		this.#staleTimer = undefined;
		this.#connectTimer = undefined;
		this.#refetchTimer = undefined;
		this.#source?.close();
		this.#source = null;
	}

	on(handler: RealtimeHandler): () => void {
		this.#handlers.add(handler);

		return () => this.#handlers.delete(handler);
	}

	refetch(...invalidated: string[]) {
		for (const key of invalidated) this.#pending.add(key);

		if (this.#refetchTimer) return;

		this.#refetchTimer = setTimeout(() => {
			this.#refetchTimer = undefined;

			void this.flush();
		}, refetchWindowMs);
	}

	flush() {
		const invalidated = [...this.#pending];

		if (invalidated.length === 0) return Promise.resolve();

		if (typeof document !== "undefined" && document.visibilityState === "hidden") {
			return Promise.resolve();
		}

		this.#pending.clear();

		return Promise.all(invalidated.map((key) => invalidate(key)));
	}

	#dispatch(kind: RealtimeEventKind, message: MessageEvent) {
		let event: RealtimeEvent;

		try {
			event = { kind, ...JSON.parse(message.data) };
		} catch {
			return;
		}

		for (const handler of this.#handlers) handler(event);
	}
}

export function invalidatedBy(event: RealtimeEvent, workspaceId: string): string[] {
	switch (event.kind) {
		case "issue.created":
		case "issue.deleted":
			return [keys.issues(workspaceId), keys.triage(workspaceId), keys.workspaceScope(workspaceId)];
		case "issue.updated":
			return [keys.issue(event.issueId), keys.issues(workspaceId)];
		case "comment.posted":
		case "comment.edited":
		case "comment.deleted":
			return [keys.issue(event.issueId)];
		case "notification.arrived":
			return [keys.inbox(workspaceId), keys.workspaceScope(workspaceId)];
		case "membership.changed":
			return [keys.members(workspaceId), keys.workspaceScope(workspaceId)];
		case "execution.updated":
			return [keys.issue(event.issueId), keys.issues(workspaceId)];
		case "execution.event":
		case "execution.changeset":
			return [];
		case "question.asked":
		case "question.settled":
			return [keys.issue(event.issueId)];
	}
}

const eventKinds: RealtimeEventKind[] = [
	"issue.created",
	"issue.updated",
	"issue.deleted",
	"comment.posted",
	"comment.edited",
	"comment.deleted",
	"notification.arrived",
	"membership.changed",
	"execution.updated",
	"execution.event",
	"execution.changeset",
	"question.asked",
	"question.settled",
];

export function provideRealtime(): RealtimeConnection {
	const connection = new RealtimeConnection();

	setContext(key, connection);

	return connection;
}

export function useRealtime(): RealtimeConnection | undefined {
	return getContext<RealtimeConnection | undefined>(key);
}
