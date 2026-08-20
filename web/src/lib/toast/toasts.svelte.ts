import { getContext, setContext } from "svelte";

const key = Symbol("norn.toasts");

const showFor = 4000;

export type Raised = {
	message: string;
	href?: string;
	action?: string;
	onaction?: () => void;
};

export class Toasts {
	#shown = $state.raw<Raised | undefined>(undefined);
	#timer: ReturnType<typeof setTimeout> | undefined;
	#held = false;

	get shown(): Raised | undefined {
		return this.#shown;
	}

	show(message: string, options: Omit<Raised, "message"> = {}) {
		this.#shown = { message, ...options };
		this.#count();
	}

	dismiss() {
		clearTimeout(this.#timer);
		this.#timer = undefined;
		this.#shown = undefined;
	}

	hold() {
		this.#held = true;
		clearTimeout(this.#timer);
	}

	release() {
		this.#held = false;

		if (this.#shown) this.#count();
	}

	stop() {
		clearTimeout(this.#timer);
	}

	#count() {
		clearTimeout(this.#timer);

		if (this.#held) return;

		this.#timer = setTimeout(() => this.dismiss(), showFor);
	}
}

export function provideToasts(): Toasts {
	return setContext(key, new Toasts());
}

export function useToasts(): Toasts {
	const toasts = getContext<Toasts | undefined>(key);

	if (!toasts) throw new Error("no toast controller is provided above this component");

	return toasts;
}
