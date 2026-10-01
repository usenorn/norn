import { describe, expect, it } from "vitest";
import { diffLineMax } from "./diff";
import { patchLines } from "./patch.server";

function zipped(text: string): Response {
	return new Response(new Response(text).body?.pipeThrough(new CompressionStream("gzip")));
}

async function collected(response: Response, stopAfter = Infinity): Promise<string[]> {
	const lines: string[] = [];

	for await (const line of patchLines(response)) {
		lines.push(line);

		if (lines.length >= stopAfter) break;
	}

	return lines;
}

describe("reading a stored patch as it streams", () => {
	const patch = ["diff --git a/x b/x", "@@ -1 +1 @@", "-old", "+new"].join("\n");

	it("reads the same lines whether the patch was stored zipped or not", async () => {
		expect(await collected(new Response(patch))).toEqual(patch.split("\n"));
		expect(await collected(zipped(patch))).toEqual(patch.split("\n"));
	});

	it("cuts a single enormous line rather than holding all of it", async () => {
		const huge = "+" + "x".repeat(diffLineMax * 50);
		const [kept] = await collected(zipped(huge + "\n+next"));

		expect(kept).toHaveLength(diffLineMax);
	});

	it("stops reading when nobody wants more", async () => {
		let pulled = 0;
		const body = new ReadableStream<Uint8Array<ArrayBuffer>>({
			pull(controller) {
				pulled += 1;
				controller.enqueue(new TextEncoder().encode(`+line ${pulled}\n`));
			},
		});

		const lines = await collected(new Response(body), 3);

		expect(lines).toHaveLength(3);
		expect(pulled).toBeLessThan(20);
	});
});
