export type DropZoneHandlers = {
	ondragenter: (event: DragEvent) => void;
	ondragover: (event: DragEvent) => void;
	ondragleave: (event: DragEvent) => void;
	ondrop: (event: DragEvent) => void;
};

export function carriesFiles(event: DragEvent): boolean {
	return Array.from(event.dataTransfer?.types ?? []).includes("Files");
}

export function dropZone(zone: {
	take: (files: File[]) => void;
	over: (dragging: boolean) => void;
	accepts?: () => boolean;
}): DropZoneHandlers {
	const open = () => zone.accepts?.() ?? true;

	let depth = 0;

	function rest() {
		depth = 0;
		zone.over(false);
	}

	return {
		ondragenter: (event) => {
			if (!carriesFiles(event)) return;

			event.preventDefault();
			depth += 1;
			zone.over(open());
		},
		ondragover: (event) => {
			if (!carriesFiles(event)) return;

			event.preventDefault();
		},
		ondragleave: (event) => {
			if (!carriesFiles(event)) return;

			depth -= 1;

			if (depth <= 0) rest();
		},
		ondrop: (event) => {
			if (!carriesFiles(event)) return;

			rest();

			const handled = event.defaultPrevented;

			event.preventDefault();

			if (handled || !open()) return;

			zone.take(Array.from(event.dataTransfer?.files ?? []));
		},
	};
}
