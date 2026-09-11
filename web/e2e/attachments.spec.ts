import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { deflateSync } from "node:zlib";
import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

function png(path: string, colour: [number, number, number]) {
	const width = 40;
	const height = 30;

	const crc = (payload: Buffer) => {
		let held = ~0;

		for (const byte of payload) {
			held ^= byte;

			for (let bit = 0; bit < 8; bit += 1) held = (held >>> 1) ^ (0xedb88320 & -(held & 1));
		}

		return ~held >>> 0;
	};

	const chunk = (kind: string, body: Buffer) => {
		const payload = Buffer.concat([Buffer.from(kind, "ascii"), body]);
		const length = Buffer.alloc(4);
		const checksum = Buffer.alloc(4);

		length.writeUInt32BE(body.length);
		checksum.writeUInt32BE(crc(payload));

		return Buffer.concat([length, payload, checksum]);
	};

	const header = Buffer.alloc(13);

	header.writeUInt32BE(width, 0);
	header.writeUInt32BE(height, 4);
	header[8] = 8;
	header[9] = 2;

	const row = Buffer.concat([Buffer.from([0]), Buffer.alloc(width * 3).fill(Buffer.from(colour))]);
	const pixels = deflateSync(Buffer.concat(Array.from({ length: height }, () => row)));

	writeFileSync(
		path,
		Buffer.concat([
			Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
			chunk("IHDR", header),
			chunk("IDAT", pixels),
			chunk("IEND", Buffer.alloc(0)),
		])
	);
}

test("several files land in the description, in the order they were picked", async ({ page }) => {
	const held = mkdtempSync(join(tmpdir(), "norn-attachments-"));
	const names = ["first.png", "second.png", "third.png"];
	const colours: [number, number, number][] = [
		[220, 60, 60],
		[60, 160, 220],
		[120, 200, 120],
	];

	names.forEach((name, index) => png(join(held, name), colours[index]));

	await page.goto(at(`/issues/${fixture().issues[0].reference}`));
	await page.getByRole("button", { name: "Edit" }).first().click();
	await page.getByRole("textbox", { name: "Description" }).click();

	await page
		.locator('input[type=file][aria-hidden="true"]')
		.first()
		.setInputFiles(names.map((name) => join(held, name)));

	const written = page.locator('[aria-label="Description"] img');

	await expect(written).toHaveCount(names.length);

	expect(await written.evaluateAll((found) => found.map((one) => one.getAttribute("alt")))).toEqual(
		names
	);

	await expect(page.locator('[aria-label="Description"] .animate-pulse')).toHaveCount(0);
});
