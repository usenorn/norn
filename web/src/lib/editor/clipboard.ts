const extensions: Record<string, string> = {
	"image/png": "png",
	"image/jpeg": "jpg",
	"image/gif": "gif",
	"image/webp": "webp",
	"image/avif": "avif",
	"image/heic": "heic",
};

const unnamed = /^image\.(png|jpe?g|gif|webp|avif|heic)$/i;

export function pastedFiles(data: DataTransfer | null | undefined, now = new Date()): File[] {
	if (!data) return [];

	// Rich text carries its pictures as markup with an address of their own, so taking the files
	// beside it would upload a copy of something the document can already point at. A screenshot
	// arrives on its own, with no text/html to go with it.
	if ([...data.types].includes("text/html")) return [];

	const held = [...(data.items ?? [])]
		.filter((item) => item.kind === "file")
		.map((item) => item.getAsFile())
		.filter((file): file is File => file !== null);

	const found = held.length > 0 ? held : [...(data.files ?? [])];

	return found.filter((file) => file.size > 0).map((file) => named(file, now));
}

function named(file: File, now: Date): File {
	if (file.name !== "" && !unnamed.test(file.name)) return file;

	const extension = extensions[file.type] ?? "png";

	return new File([file], `${screenshotName(now)}.${extension}`, {
		type: file.type,
		lastModified: file.lastModified,
	});
}

function screenshotName(now: Date): string {
	const two = (value: number) => String(value).padStart(2, "0");

	const day = `${now.getFullYear()}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
	const time = `${two(now.getHours())}${two(now.getMinutes())}${two(now.getSeconds())}`;

	return `screenshot-${day}-${time}`;
}
