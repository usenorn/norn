export const avatarTones = [
	"cyan",
	"blue",
	"violet",
	"orchid",
	"magenta",
	"rose",
	"amber",
	"green",
] as const;

export type AvatarTone = (typeof avatarTones)[number];

const fnvOffset = 0x811c9dc5;
const fnvPrime = 0x01000193;

export function avatarToneOf(accountId: string): AvatarTone {
	let hash = fnvOffset;

	for (const unit of accountId.toLowerCase()) {
		hash ^= unit.codePointAt(0) ?? 0;
		hash = Math.imul(hash, fnvPrime);
	}

	return avatarTones[(hash >>> 0) % avatarTones.length];
}
