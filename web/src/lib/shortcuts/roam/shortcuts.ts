import type { Shortcut } from "../shortcuts";

export const roamMode = "roam";

export const roamShortcuts = [
	{ id: "roam-toggle", keys: ["`"], label: "Move around with W A S D", group: "global" },
	{ id: "roam-up", keys: ["w"], label: "Up", group: "roam", mode: roamMode },
	{ id: "roam-down", keys: ["s"], label: "Down", group: "roam", mode: roamMode },
	{ id: "roam-left", keys: ["a"], label: "Left", group: "roam", mode: roamMode },
	{ id: "roam-right", keys: ["d"], label: "Right", group: "roam", mode: roamMode },
	{ id: "roam-enter", keys: ["enter"], label: "Open", group: "roam", mode: roamMode },
	{ id: "roam-leave", keys: ["escape"], label: "Stop moving", group: "roam", mode: roamMode },
] as const satisfies readonly Shortcut[];
