import { describe, expect, it } from 'vitest';

import { caretAfterPaste, pastedMarkdown, withPasted } from './paste';

function clipboard(data: Record<string, string>, files: File[] = []): ClipboardEvent {
	return {
		clipboardData: {
			files,
			getData: (kind: string) => data[kind] ?? ''
		}
	} as unknown as ClipboardEvent;
}

describe('pasting into a description', () => {
	it('keeps the structure a rich source carries', () => {
		const markdown = pastedMarkdown(
			clipboard({
				'text/html': '<h2>Steps</h2><ul><li>Press <strong>Export</strong></li></ul>',
				'text/plain': 'Steps\nPress Export'
			})
		);

		expect(markdown).toContain('## Steps');
		expect(markdown).toMatch(/^- +Press \*\*Export\*\*$/m);
	});

	it('leaves plain text alone', () => {
		expect(
			pastedMarkdown(clipboard({ 'text/html': '<p>Just words</p>', 'text/plain': 'Just words' }))
		).toBeNull();
	});

	it('yields to files, which are attachments rather than text', () => {
		const file = new File(['x'], 'shot.png', { type: 'image/png' });

		expect(pastedMarkdown(clipboard({ 'text/html': '<p>x</p>' }, [file]))).toBeNull();
	});
});

describe('placing pasted markdown', () => {
	const field = (value: string, start: number, end: number) =>
		({ value, selectionStart: start, selectionEnd: end }) as HTMLTextAreaElement;

	it('replaces the selection rather than appending', () => {
		expect(withPasted(field('one two', 4, 7), 'three')).toBe('one three');
	});

	it('leaves the caret after what was pasted', () => {
		expect(caretAfterPaste(field('one two', 4, 7), 'three')).toBe(9);
	});
});
