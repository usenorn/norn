export async function invalidate(_key: string | URL | ((url: URL) => boolean)): Promise<void> {}

export async function invalidateAll(): Promise<void> {}
