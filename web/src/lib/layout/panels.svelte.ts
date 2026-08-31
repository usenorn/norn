import { getContext, setContext } from "svelte";
import {
	clampPanel,
	panelBounds,
	panelCookie,
	writePanel,
	type PanelName,
	type PanelSize,
	type PanelSizes,
} from "./panels";

const key = Symbol("norn.panels");

const remembered = 60 * 60 * 24 * 365;

export class Panels {
	#sizes = $state.raw<PanelSizes>({
		sidebar: { width: panelBounds("sidebar").ordinary, collapsed: false },
		properties: { width: panelBounds("properties").ordinary, collapsed: false },
	});

	constructor(sizes: PanelSizes) {
		this.#sizes = sizes;
	}

	size(name: PanelName): PanelSize {
		return this.#sizes[name];
	}

	width(name: PanelName): number {
		return this.#sizes[name].width;
	}

	collapsed(name: PanelName): boolean {
		return this.#sizes[name].collapsed;
	}

	resize(name: PanelName, width: number, room?: number) {
		this.#remember(name, { width: clampPanel(name, width, room), collapsed: false });
	}

	collapse(name: PanelName) {
		this.#remember(name, { ...this.#sizes[name], collapsed: true });
	}

	restore(name: PanelName) {
		this.#remember(name, { ...this.#sizes[name], collapsed: false });
	}

	#remember(name: PanelName, size: PanelSize) {
		this.#sizes = { ...this.#sizes, [name]: size };

		if (typeof document === "undefined") return;

		document.cookie = `${panelCookie(name)}=${writePanel(size)};path=/;max-age=${remembered};samesite=lax`;
	}
}

export function providePanels(sizes: PanelSizes): Panels {
	return setContext(key, new Panels(sizes));
}

export function usePanels(): Panels {
	const panels = getContext<Panels | undefined>(key);

	if (!panels) throw new Error("no panel sizes are provided above this component");

	return panels;
}
