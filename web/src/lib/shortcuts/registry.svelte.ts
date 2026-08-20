import { getContext, onDestroy, setContext } from "svelte";
import {
	isActivatableTarget,
	isTypingTarget,
	shortcutOf,
	shortcuts,
	type Shortcut,
	type ShortcutId,
} from "./shortcuts";

const key = Symbol("norn.shortcuts");

const sequenceTimeout = 1200;

type Handler = (binding: string) => void;

function chordOf(event: KeyboardEvent): string {
	const pressed = event.key.toLowerCase();
	const parts: string[] = [];

	if (event.metaKey || event.ctrlKey) parts.push("mod");
	if (event.shiftKey && pressed !== "?" && pressed !== " ") parts.push("shift");

	parts.push(pressed);

	return parts.join("+");
}

export class ShortcutRegistry {
	#bound = new Map<string, Handler>();
	#holds = 0;
	#pending = "";
	#expires = 0;
	#changed = $state.raw({});
	#mode = $state.raw<string | null>(null);

	hold(): () => void {
		this.#holds += 1;

		let released = false;

		return () => {
			if (released) return;

			released = true;
			this.#holds -= 1;
		};
	}

	get mode(): string | null {
		return this.#mode;
	}

	enterMode(mode: string) {
		this.#mode = mode;
	}

	leaveMode() {
		this.#mode = null;
	}

	register(id: ShortcutId, handler: Handler): () => void {
		const shortcut = shortcutOf(id);

		if (import.meta.env.DEV) this.#refuseCollision(shortcut);

		this.#bound.set(id, handler);
		this.#changed = {};

		return () => {
			if (this.#bound.get(id) !== handler) return;

			this.#bound.delete(id);
			this.#changed = {};
		};
	}

	get active(): Shortcut[] {
		void this.#changed;

		return shortcuts.filter(
			(shortcut) =>
				this.#bound.has(shortcut.id) && (!shortcut.mode || shortcut.mode === this.#mode)
		);
	}

	bound(id: ShortcutId): boolean {
		void this.#changed;

		return this.#bound.has(id);
	}

	handle(event: KeyboardEvent): boolean {
		if (event.isComposing || event.repeat || this.#holds > 0) return false;

		const chord = chordOf(event);
		const typing = isTypingTarget(event.target);
		const focused = isActivatableTarget(event.target);

		if (this.#pending && Date.now() < this.#expires) {
			const sequence = `${this.#pending} ${chord}`;

			this.#pending = "";

			if (this.#run(sequence, typing, focused)) {
				event.preventDefault();

				return true;
			}
		}

		this.#pending = "";

		if (!typing && this.#prefixes().has(chord)) {
			this.#pending = chord;
			this.#expires = Date.now() + sequenceTimeout;
			event.preventDefault();

			return true;
		}

		if (this.#run(chord, typing, focused)) {
			event.preventDefault();

			return true;
		}

		return false;
	}

	#declared(): Shortcut[] {
		const inMode = shortcuts.filter(
			(shortcut) => this.#bound.has(shortcut.id) && shortcut.mode === this.#mode
		);

		const outside = shortcuts.filter(
			(shortcut) => this.#bound.has(shortcut.id) && !shortcut.mode
		);

		return this.#mode ? [...inMode, ...outside] : outside;
	}

	#run(binding: string, typing: boolean, focused: boolean): boolean {
		for (const shortcut of this.#declared()) {
			if (typing && !shortcut.whileTyping) continue;
			if (focused && shortcut.yieldsToFocus) continue;
			if (!shortcut.keys.includes(binding)) continue;

			this.#bound.get(shortcut.id)?.(binding);

			return true;
		}

		return false;
	}

	#prefixes(): Set<string> {
		const found = new Set<string>();

		for (const shortcut of this.#declared()) {
			for (const binding of shortcut.keys) {
				const [first, ...rest] = binding.split(" ");

				if (rest.length > 0) found.add(first);
			}
		}

		return found;
	}

	#refuseCollision(shortcut: Shortcut) {
		for (const other of shortcuts) {
			if (other.id === shortcut.id) continue;
			if (!this.#bound.has(other.id)) continue;
			if (other.mode !== shortcut.mode) continue;

			const clash = other.keys.find((binding) => shortcut.keys.includes(binding));

			if (clash) {
				throw new Error(
					`${shortcut.id} and ${other.id} both want ${clash}; one screen would silently win`
				);
			}
		}
	}
}

export function provideShortcuts(): ShortcutRegistry {
	return setContext(key, new ShortcutRegistry());
}

export function useShortcuts(): ShortcutRegistry {
	const registry = getContext<ShortcutRegistry | undefined>(key);

	if (!registry) throw new Error("no shortcut registry is provided above this component");

	return registry;
}

export function bindShortcut(id: ShortcutId, handler: Handler) {
	const registry = useShortcuts();
	const release = registry.register(id, handler);

	onDestroy(release);
}

export function holdShortcuts(active: () => boolean) {
	const registry = useShortcuts();

	$effect(() => {
		if (!active()) return;

		return registry.hold();
	});
}

export function bindShortcuts(bindings: Partial<Record<ShortcutId, Handler>>) {
	const registry = useShortcuts();
	const released: Array<() => void> = [];

	for (const [id, handler] of Object.entries(bindings)) {
		if (handler) released.push(registry.register(id as ShortcutId, handler));
	}

	onDestroy(() => released.forEach((release) => release()));
}
