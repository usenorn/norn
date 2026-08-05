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

/** How long a drop is tolerated before the screen admits it may be showing stale data. */
export const staleAfterMs = 30_000;

/** Bursts are coalesced so a bulk change is one refetch rather than one per issue. */
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
			// A closed source means the browser gave up rather than scheduled a retry, which is
			// what an instance with live updates turned off looks like from here.
			if (source.readyState === EventSource.CLOSED) {
				this.state = "off";
				clearTimeout(this.#staleTimer);

				return;
			}

			// EventSource reconnects on its own, so a blip must not shout. Only a drop that lasts
			// long enough for the screen to be plausibly wrong is worth interrupting anybody over.
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

	/** Registers a handler for as long as the caller lives; the returned function unregisters it. */
	on(handler: RealtimeHandler): () => void {
		this.#handlers.add(handler);

		return () => this.#handlers.delete(handler);
	}

	/**
	 * Asks for a refetch, coalescing anything that arrives in the same window. A filtered list
	 * cannot be patched from an event because only the server knows whether a changed issue still
	 * belongs in it; the filter language exists as Go that compiles to SQL.
	 */
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
