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
	| "membership.changed";

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

export const refetchWindowMs = 400;

const key = Symbol("norn.realtime");

export class RealtimeConnection {
	state = $state<RealtimeState>("connecting");

	#source: EventSource | null = null;
	#handlers = new Set<RealtimeHandler>();
	#staleTimer: ReturnType<typeof setTimeout> | undefined;
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

		const query = new URLSearchParams({ topics });

		if (slot) query.set(sessionParam, slot);

		const source = new EventSource(`/v1/workspaces/${workspaceId}/events?${query}`);

		source.onopen = () => {
			clearTimeout(this.#staleTimer);
			this.state = "live";
		};

		source.onerror = () => {
			if (source.readyState === EventSource.CLOSED) {
				this.state = "off";
				clearTimeout(this.#staleTimer);

				return;
			}

			if (this.state !== "stale") this.state = "reconnecting";

			clearTimeout(this.#staleTimer);
			this.#staleTimer = setTimeout(() => (this.state = "stale"), staleAfterMs);
		};

		source.addEventListener("resync", () => void invalidateAll());

		for (const kind of eventKinds) {
			source.addEventListener(kind, (message) => this.#dispatch(kind, message as MessageEvent));
		}

		this.#source = source;
	}

	close() {
		clearTimeout(this.#staleTimer);
		clearTimeout(this.#refetchTimer);
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
];

export function provideRealtime(): RealtimeConnection {
	const connection = new RealtimeConnection();

	setContext(key, connection);

	return connection;
}

export function useRealtime(): RealtimeConnection | undefined {
	return getContext<RealtimeConnection | undefined>(key);
}
