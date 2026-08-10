import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import type { ActingSession, SignedInAccount } from "$lib/account/accounts";

declare global {
	namespace App {
		interface Error {
			message: string;
			code?: "not_found" | "unexpected";
			reference?: string;
		}
		interface Locals {
			api: Client<paths>;
			apiAs: (slot: string) => Client<paths>;
			signedIn: Promise<SignedInAccount[]>;
			acting: Promise<string | null>;
			correlationId: string;
		}
		interface PageData {
			acting?: ActingSession;
		}
	}
}

export {};
