import { Extension } from "@tiptap/core";
import Suggestion, { type SuggestionOptions } from "@tiptap/suggestion";
import { PluginKey } from "@tiptap/pm/state";

export type SuggestionAnchor = { getBoundingClientRect: () => DOMRect };

export type SuggestionSession = {
	query: string;
	anchor: SuggestionAnchor;
	take: (chosen: unknown) => void;
};

export type SuggestionHandlers = {
	onOpen: (session: SuggestionSession) => void;
	onQuery: (session: SuggestionSession) => void;
	onKey: (event: KeyboardEvent) => boolean;
	onClose: () => void;
};

export function completing(
	name: string,
	char: string,
	handlers: SuggestionHandlers,
	options: Partial<SuggestionOptions> & { opensAfter?: (before: string) => boolean } = {}
) {
	const { opensAfter, ...suggestion } = options;
	return Extension.create({
		name: `editor-${name}`,
		addProseMirrorPlugins() {
			return [
				Suggestion({
					editor: this.editor,
					char,
					pluginKey: new PluginKey(`editor-${name}`),
					items: () => [],
					allow: ({ state, range }) => {
						const before = state.doc.textBetween(
							Math.max(range.from - 1, 0),
							Math.max(range.from, 0),
							undefined,
							"￼"
						);

						return opensAfter ? opensAfter(before) : true;
					},
					render: () => ({
						onStart: (props) => handlers.onOpen(sessionOf(props)),
						onUpdate: (props) => handlers.onQuery(sessionOf(props)),
						onKeyDown: ({ event }) => handlers.onKey(event),
						onExit: () => handlers.onClose(),
					}),
					...suggestion,
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
	const measure = props.clientRect;

	return {
		query: props.query,
		anchor: { getBoundingClientRect: () => measure?.() ?? new DOMRect() },
		take: props.command,
	};
}
