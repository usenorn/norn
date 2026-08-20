import { getContext, setContext } from "svelte";
import type { CreationOutcome } from "./creating";
import type { Issue } from "./issues";
import type { NewIssueInput } from "./new-issue-schema";

const key = Symbol("norn.new-issue");

export type NewIssueSurface = {
	seed?: Partial<NewIssueInput>;
	onraising?: (key: string, draft: Issue) => void;
	onsettled?: (outcome: CreationOutcome) => void | Promise<void>;
};

export class NewIssue {
	#open = $state(false);
	#prefill = $state.raw<Partial<NewIssueInput>>({});
	#surface = $state.raw<NewIssueSurface>({});

	get open(): boolean {
		return this.#open;
	}

	set open(next: boolean) {
		this.#open = next;
	}

	get prefill(): Partial<NewIssueInput> {
		return this.#prefill;
	}

	get onraising(): NewIssueSurface["onraising"] {
		return this.#surface.onraising;
	}

	get onsettled(): NewIssueSurface["onsettled"] {
		return this.#surface.onsettled;
	}

	raise(seed?: Partial<NewIssueInput>) {
		this.#prefill = { ...this.#surface.seed, ...seed };
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
