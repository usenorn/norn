import { browser } from "$app/environment";

/**
 * Shortcut hints are read, not pressed, so they have to name the key the reader's own keyboard
 * has. Guessing from the browser is the only signal available and it is wrong on a Mac keyboard
 * plugged into a PC; being wrong there is better than showing Ctrl to everybody on a Mac.
 */
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

/**
 * A key pressed to choose a character in an input method is not a key pressed at the document.
 * Reading one as the other cuts a word in half in Japanese, Chinese and Korean, so every
 * handler that acts on a keystroke asks this first.
 */
export function composing(event: KeyboardEvent): boolean {
	return event.isComposing || event.keyCode === 229;
}
