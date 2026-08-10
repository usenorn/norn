import { getContext, onDestroy, setContext } from "svelte";
import { useShortcuts } from "../registry.svelte";
import { roamMode } from "./shortcuts";

const key = Symbol("norn.roam");

export type RoamSurface = {
	up: () => void;
	down: () => void;
	left: () => boolean;
	right: () => boolean;
	enter: () => void;
};

const navSelector = '[data-roam="nav"] a[href]';

export class Roam {
	#surface = $state.raw<RoamSurface | undefined>(undefined);
	#onNav = $state(false);

	get roaming(): boolean {
		return this.#onNav || Boolean(this.#surface);
	}

	get onNav(): boolean {
		return this.#onNav;
	}

	attach(surface: RoamSurface): () => void {
		this.#surface = surface;

		return () => {
			if (this.#surface === surface) this.#surface = undefined;
		};
	}

	start() {
		this.#onNav = false;
	}

	stop() {
		this.#leaveNav();
	}

	up() {
		if (this.#onNav) return this.#stepNav(-1);

		this.#surface?.up();
	}

	down() {
		if (this.#onNav) return this.#stepNav(1);

		this.#surface?.down();
	}

	left() {
		if (this.#onNav) return;

		if (this.#surface?.left() === false || !this.#surface) this.#enterNav();
	}

	right() {
		if (this.#onNav) {
			this.#leaveNav();

			return;
		}

		this.#surface?.right();
	}

	enter() {
		if (this.#onNav) {
			const focused = document.activeElement;

			if (focused instanceof HTMLElement) focused.click();

			return;
		}

		this.#surface?.enter();
	}

	#leaveNav() {
		if (!this.#onNav) return;

		this.#onNav = false;

		const focused = document.activeElement;

		if (focused instanceof HTMLElement && focused.closest('[data-roam="nav"]')) focused.blur();
	}

	#links(): HTMLAnchorElement[] {
		return Array.from(document.querySelectorAll<HTMLAnchorElement>(navSelector));
	}

	#enterNav() {
		const links = this.#links();

		if (links.length === 0) return;

		this.#onNav = true;

		const current = links.findIndex((link) => link.getAttribute("aria-current"));

		links[current >= 0 ? current : 0].focus();
	}

	#stepNav(by: number) {
		const links = this.#links();

		if (links.length === 0) return;

		const at = links.indexOf(document.activeElement as HTMLAnchorElement);
		const next = Math.min(Math.max((at < 0 ? 0 : at) + by, 0), links.length - 1);

		links[next].focus();
	}
}

export function provideRoam(): Roam {
	return setContext(key, new Roam());
}

export function useRoam(): Roam {
	const roam = getContext<Roam | undefined>(key);

	if (!roam) throw new Error("no roam controller is provided above this component");

	return roam;
}

export function registerRoam(surface: () => RoamSurface) {
	const roam = useRoam();

	$effect(() => roam.attach(surface()));
}

export function bindRoam(roam: Roam) {
	const shortcuts = useShortcuts();

	const released = [
		shortcuts.register("roam-toggle", () => {
			if (shortcuts.mode === roamMode) {
				shortcuts.leaveMode();
				roam.stop();

				return;
			}

			shortcuts.enterMode(roamMode);
			roam.start();
		}),
		shortcuts.register("roam-leave", () => {
			shortcuts.leaveMode();
			roam.stop();
		}),
		shortcuts.register("roam-up", () => roam.up()),
		shortcuts.register("roam-down", () => roam.down()),
		shortcuts.register("roam-left", () => roam.left()),
		shortcuts.register("roam-right", () => roam.right()),
		shortcuts.register("roam-enter", () => roam.enter()),
	];

	onDestroy(() => released.forEach((release) => release()));
}
