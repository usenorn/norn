import { DiffReader, diffLineMax, type DiffBudget } from "./diff";

const gzipMagic = [0x1f, 0x8b];

type Bytes = Uint8Array<ArrayBuffer>;

function replayed(first: Bytes, rest: ReadableStreamDefaultReader<Bytes>): ReadableStream<BufferSource> {
	return new ReadableStream<BufferSource>({
		start(controller) {
			controller.enqueue(first);
		},
		async pull(controller) {
			const { done, value } = await rest.read();

			if (done) controller.close();
			else controller.enqueue(value);
		},
		cancel(reason) {
			return rest.cancel(reason);
		},
	});
}

async function textOf(body: ReadableStream<Bytes>): Promise<ReadableStream<string> | null> {
	const reader = body.getReader();
	const { done, value } = await reader.read();

	if (done || !value) return null;

	const bytes = replayed(value, reader);
	const zipped = value[0] === gzipMagic[0] && value[1] === gzipMagic[1];
	const plain: ReadableStream<BufferSource> = zipped ? bytes.pipeThrough(new DecompressionStream("gzip")) : bytes;

	return plain.pipeThrough(new TextDecoderStream());
}

export async function* patchLines(stored: Response): AsyncGenerator<string> {
	if (!stored.body) return;

	const text = await textOf(stored.body);

	if (!text) return;

	const reader = text.getReader();

	let line = "";
	let clipped = false;

	try {
		for (;;) {
			const { done, value } = await reader.read();

			if (done) break;

			let from = 0;

			for (;;) {
				const end = value.indexOf("\n", from);
				const piece = value.slice(from, end === -1 ? undefined : end);

				if (!clipped) {
					line += piece;

					if (line.length > diffLineMax) {
						line = line.slice(0, diffLineMax);
						clipped = true;
					}
				}

				if (end === -1) break;

				yield line;

				line = "";
				clipped = false;
				from = end + 1;
			}
		}

		if (line !== "") yield line;
	} finally {
		await reader.cancel().catch(() => undefined);
	}
}

export async function readPatch(stored: Response, budget: DiffBudget): Promise<DiffReader> {
	const reader = new DiffReader(budget);

	for await (const line of patchLines(stored)) {
		reader.push(line);

		if (reader.done) break;
	}

	reader.finish();

	return reader;
}
