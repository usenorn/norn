import { Extension } from "@tiptap/core";
import Suggestion, { type SuggestionOptions } from "@tiptap/suggestion";
import { PluginKey } from "@tiptap/pm/state";
import { opensMenu } from "$lib/editor/slash";

export type SuggestionSession = {
	query: string;
	rect: DOMRect;
	take: (chosen: unknown) => void;
};

export type SuggestionHandlers = {
	onOpen: (session: SuggestionSession) => void;
	onQuery: (session: SuggestionSession) => void;
	onKey: (event: KeyboardEvent) => boolean;
	onClose: () => void;
};

/**
 * completing wires one trigger character to a popup the caller draws. The popup never takes
 * focus: the caret stays in the document so typing keeps working and a screen reader keeps
 * reading the text rather than the list.
 */
export function completing(
	name: string,
	char: string,
	handlers: SuggestionHandlers,
	options: Partial<SuggestionOptions> = {}
) {
	return Extension.create({
		name: `editor-${name}`,
		addProseMirrorPlugins() {
			return [
				Suggestion({
					editor: this.editor,
					char,
					pluginKey: new PluginKey(`editor-${name}`),
					allowSpaces: char !== "/",
					items: () => [],
					allow: ({ state, range }) => {
						const before = state.doc.textBetween(
							Math.max(range.from - 1, 0),
							Math.max(range.from, 0),
							undefined,
							"￼"
						);

						return char === "/" ? opensMenu(before) : true;
					},
					render: () => ({
						onStart: (props) => handlers.onOpen(sessionOf(props)),
						onUpdate: (props) => handlers.onQuery(sessionOf(props)),
						onKeyDown: ({ event }) => handlers.onKey(event),
						onExit: () => handlers.onClose(),
					}),
					...options,
				}),
			];
		},
	});
}

function sessionOf(props: {
	query: string;
	command: (chosen: unknown) => void;
	clientRect?: (() => DOMRect | null) | null;
}): SuggestionSession {
	return {
		query: props.query,
		rect: props.clientRect?.() ?? new DOMRect(),
		take: props.command,
	};
}
