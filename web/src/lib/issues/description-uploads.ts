import { Extension } from "@tiptap/core";
import { Plugin, PluginKey, type EditorState } from "@tiptap/pm/state";
import { Decoration, DecorationSet, type EditorView } from "@tiptap/pm/view";

export type UploadPlaceholder = {
	id: string;
	name: string;
	preview: string | null;
};

type Placement = { add: UploadPlaceholder & { pos: number } } | { remove: string };

const key = new PluginKey<DecorationSet>("describe-uploads");

const imageClass =
	"pointer-events-none inline-block max-w-full animate-pulse opacity-60 select-none";
const chipClass =
	"pointer-events-none inline-flex max-w-full animate-pulse items-center rounded-sm border border-line-subtle bg-paper-2 px-1.5 py-0.5 align-baseline text-sm text-muted-foreground select-none";

function render(placeholder: UploadPlaceholder): HTMLElement {
	if (placeholder.preview) {
		const image = document.createElement("img");

		image.src = placeholder.preview;
		image.alt = placeholder.name;
		image.className = imageClass;
		image.draggable = false;

		return image;
	}

	const chip = document.createElement("span");

	chip.className = chipClass;
	chip.textContent = placeholder.name;

	return chip;
}

function widget(placement: UploadPlaceholder & { pos: number }): Decoration {
	return Decoration.widget(placement.pos, () => render(placement), {
		id: placement.id,
		side: -1,
		destroy: () => {
			if (placement.preview) URL.revokeObjectURL(placement.preview);
		},
	});
}

function held(set: DecorationSet, id: string): Decoration[] {
	return set.find(undefined, undefined, (spec) => spec.id === id);
}

export const DescriptionUploads = Extension.create({
	name: "describe-uploads",

	addProseMirrorPlugins() {
		return [
			new Plugin<DecorationSet>({
				key,
				state: {
					init: () => DecorationSet.empty,
					apply(transaction, previous) {
						const mapped = previous.map(transaction.mapping, transaction.doc);
						const placement = transaction.getMeta(key) as Placement | undefined;

						if (!placement) return mapped;
						if ("remove" in placement) return mapped.remove(held(mapped, placement.remove));

						return mapped.add(transaction.doc, [widget(placement.add)]);
					},
				},
				props: {
					decorations: (state) => key.getState(state),
				},
			}),
		];
	},
});

export function previewOf(file: File): string | null {
	return file.type.startsWith("image/") ? URL.createObjectURL(file) : null;
}

export function placeUpload(view: EditorView, placeholder: UploadPlaceholder, pos: number) {
	view.dispatch(view.state.tr.setMeta(key, { add: { ...placeholder, pos } }));
}

export function uploadPosition(state: EditorState, id: string): number | null {
	const [found] = held(key.getState(state) ?? DecorationSet.empty, id);

	return found ? found.from : null;
}

export function dropUpload(view: EditorView, id: string) {
	if (uploadPosition(view.state, id) === null) return;

	view.dispatch(view.state.tr.setMeta(key, { remove: id }));
}
