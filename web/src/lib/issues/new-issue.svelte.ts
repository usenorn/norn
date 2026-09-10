import { getContext, setContext } from "svelte";
import type { CreationOutcome } from "./creating";
import type { Issue } from "./issues";
import type { NewIssuePrefill } from "./new-issue-schema";

const key = Symbol("norn.new-issue");

export type NewIssueSurface = {
	seed?: NewIssuePrefill;
	onraising?: (key: string, draft: Issue) => void;
	onsettled?: (outcome: CreationOutcome) => void | Promise<void>;
};

export class NewIssue {
	#open = $state(false);
	#prefill = $state.raw<NewIssuePrefill>({});
	#surface = $state.raw<NewIssueSurface>({});
	#held = $state.raw<NewIssueSurface>({});

	get open(): boolean {
		return this.#open;
	}

	set open(next: boolean) {
		this.#open = next;
	}

	get prefill(): NewIssuePrefill {
		return this.#prefill;
	}

	get onraising(): NewIssueSurface["onraising"] {
		return this.#held.onraising;
	}

	get onsettled(): NewIssueSurface["onsettled"] {
		return this.#held.onsettled;
	}

	raise(seed?: NewIssuePrefill) {
		this.#prefill = { ...this.#surface.seed, ...seed };
		this.#held = this.#surface;
		this.#open = true;
	}

	attach(surface: NewIssueSurface): () => void {
		this.#surface = surface;

		return () => {
			if (this.#surface === surface) this.#surface = {};
		};
	}
}

export function provideNewIssue(): NewIssue {
	return setContext(key, new NewIssue());
}

export function useNewIssue(): NewIssue {
	const raising = getContext<NewIssue | undefined>(key);

	if (!raising) throw new Error("no new-issue controller is provided above this component");

	return raising;
}

export function registerNewIssue(surface: () => NewIssueSurface) {
	const raising = useNewIssue();

	$effect(() => raising.attach(surface()));
}
