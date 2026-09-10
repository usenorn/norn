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

export function withModifier(event: KeyboardEvent): boolean {
	return onApple() ? event.metaKey : event.ctrlKey;
}

export function composing(event: KeyboardEvent): boolean {
	return event.isComposing || event.keyCode === 229;
}
