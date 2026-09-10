import { TaskItem } from "@tiptap/extension-list";
import { Plugin, PluginKey } from "@tiptap/pm/state";

const naming = new PluginKey("editor-task-names");

function minted(): string {
	return crypto.randomUUID();
}

export const NamedTaskItem = TaskItem.extend({
	addAttributes() {
		return {
			...this.parent?.(),
			id: {
				default: null,
				parseHTML: (element) => element.getAttribute("data-id"),
				renderHTML: (attributes) =>
					attributes.id ? { "data-id": attributes.id } : {},
			},
		};
	},

	addProseMirrorPlugins() {
		return [
			...(this.parent?.() ?? []),
			new Plugin({
				key: naming,
				appendTransaction: (_transactions, _before, state) => {
					const seen = new Set<string>();
					const unnamed: number[] = [];

					state.doc.descendants((node, pos) => {
						if (node.type.name !== "taskItem") return;

						const held = node.attrs.id;

						if (typeof held === "string" && held !== "" && !seen.has(held)) {
							seen.add(held);

							return;
						}

						unnamed.push(pos);
					});

					if (unnamed.length === 0) return null;

					const transaction = state.tr;

					for (const pos of unnamed) {
						const node = state.doc.nodeAt(pos);

						if (!node) continue;

						transaction.setNodeMarkup(pos, undefined, { ...node.attrs, id: minted() });
					}

					return transaction.setMeta("addToHistory", false);
				},
			}),
		];
	},
});
