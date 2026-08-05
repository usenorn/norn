import { getContext, setContext } from "svelte";
import { invalidateAll } from "$app/navigation";
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
	#workspaceId = "";
	#topics = "workspace,inbox";

	get connected(): boolean {
		return this.state === "live";
	}

	get degraded(): boolean {
		return this.state === "stale" || this.state === "off";
	}

	open(workspaceId: string, topics = "workspace,inbox") {
		if (typeof EventSource === "undefined") {
			this.state = "off";

			return;
		}

		if (this.#source && this.#workspaceId === workspaceId && this.#topics === topics) return;

		this.close();

		this.#workspaceId = workspaceId;
		this.#topics = topics;
		this.state = "connecting";

		const source = new EventSource(
			`/v1/workspaces/${workspaceId}/events?topics=${encodeURIComponent(topics)}`
		);

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
		this.#source?.close();
		this.#source = null;
	}

	on(handler: RealtimeHandler): () => void {
		this.#handlers.add(handler);

		return () => this.#handlers.delete(handler);
	}

	refetch() {
		clearTimeout(this.#refetchTimer);
		this.#refetchTimer = setTimeout(() => void invalidateAll(), refetchWindowMs);
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
