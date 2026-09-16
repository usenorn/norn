import { getContext, setContext, untrack } from "svelte";
import type { CommandScope, IssueTarget } from "./model";

const key = Symbol("norn.command-targets");

export type TargetSlot = "selection" | "open";

export type CommandSurface = {
	issues: IssueTarget[];
	onapplied?: () => void | Promise<void>;
};

export class CommandTargets {
	#surfaces = $state.raw<Partial<Record<TargetSlot, CommandSurface>>>({});

	get surface(): CommandSurface | undefined {
		const { selection, open } = this.#surfaces;

		if (selection && selection.issues.length > 0) return selection;
		if (open && open.issues.length > 0) return open;

		return undefined;
	}

	get scope(): CommandScope {
		const surface = this.surface;

		return surface ? { kind: "issues", issues: surface.issues } : { kind: "none" };
	}

	attach(slot: TargetSlot, surface: CommandSurface): () => void {
		untrack(() => (this.#surfaces = { ...this.#surfaces, [slot]: surface }));

		return () =>
			untrack(() => {
				if (this.#surfaces[slot] !== surface) return;

				const remaining = { ...this.#surfaces };

				delete remaining[slot];
				this.#surfaces = remaining;
			});
	}
}

export function provideCommandTargets(): CommandTargets {
	return setContext(key, new CommandTargets());
}

export function useCommandTargets(): CommandTargets {
	const targets = getContext<CommandTargets | undefined>(key);

	if (!targets) throw new Error("no command targets are provided above this component");

	return targets;
}

export function registerCommandTargets(slot: TargetSlot, surface: () => CommandSurface) {
	const targets = useCommandTargets();

	$effect(() => targets.attach(slot, surface()));
}
