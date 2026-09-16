import { readStored, store } from "$lib/storage";

export type Density = "comfortable" | "compact";

const storageKey = "norn.density";

export function readDensity(): Density {
	return readStored(storageKey) === "compact" ? "compact" : "comfortable";
}

export function applyDensity(density: Density) {
	if (density === "compact") document.documentElement.dataset.density = density;
	else delete document.documentElement.dataset.density;
}

export function toggleDensity(): Density {
	const next: Density = readDensity() === "compact" ? "comfortable" : "compact";

	store(storageKey, next);
	applyDensity(next);

	return next;
}
