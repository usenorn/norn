import { browser } from "$app/environment";

export function onApple(): boolean {
	if (!browser) return false;

	return /mac|iphone|ipad|ipod/i.test(navigator.platform || navigator.userAgent);
}

export function modifierLabel(): string {
	return onApple() ? "⌘" : "Ctrl";
}

export function shortcutLabel(key: string): string {
	return onApple() ? `⌘${key.toUpperCase()}` : `Ctrl+${key.toUpperCase()}`;
}

const appleNames: Record<string, string> = { Mod: "⌘", Shift: "⇧", Alt: "⌥" };

export function combination(spec: string): string {
	const apple = onApple();

	const parts = spec.split("+").map((part) => {
		if (part === "Mod") return apple ? appleNames.Mod : "Ctrl";
		if (part === "Shift" || part === "Alt") return apple ? appleNames[part] : part;

		return part.length === 1 ? part.toUpperCase() : part;
	});

	return parts.join(apple ? "" : "+");
}

export function withModifier(event: KeyboardEvent): boolean {
	return onApple() ? event.metaKey : event.ctrlKey;
}

export function composing(event: KeyboardEvent): boolean {
	return event.isComposing || event.keyCode === 229;
}
